package webplay

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/playtest"
	"github.com/darwindeck/darwindeck/pkg/seeds"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

// testServer registers the first few classic seeds and returns a live test
// server over the real handler mux.
func testServer(t *testing.T) (*httptest.Server, []string) {
	t.Helper()
	var games []Game
	var ids []string
	for _, g := range seeds.All() {
		if len(games) >= 3 {
			break
		}
		game := RegisterGame(len(games), g, "seed.json")
		games = append(games, game)
		ids = append(ids, game.ID)
	}
	srv := NewServer(games)
	srv.ResultsPath = filepath.Join(t.TempDir(), "results.jsonl") // never write to the repo
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, ids
}

// serverWith builds a Server (no network) over a single seed so the hardening
// tests below can drive the mux synchronously and retune knobs (clock, TTLs,
// capacity) between calls without racing a server goroutine.
func serverWith(t *testing.T, g *genome.Genome) *Server {
	t.Helper()
	srv := NewServer([]Game{RegisterGame(0, g, "seed.json")})
	srv.ResultsPath = filepath.Join(t.TempDir(), "results.jsonl")
	return srv
}

// doJSON drives a handler synchronously via httptest.NewRequest + recorder.
func doJSON(t *testing.T, h http.Handler, method, path string, hdr map[string]string, body map[string]interface{}, out interface{}) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if out != nil && rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
	return rec.Code
}

// finishGame plays the session to a terminal state over the handler by always
// submitting the first legal move (legal by construction; the max-turns cap in
// advance() guarantees termination). v is updated to the final view.
func finishGame(t *testing.T, h http.Handler, hdr map[string]string, v *View) {
	t.Helper()
	for i := 0; i < 100000 && v.Status == StatusHumanTurn; i++ {
		if code := doJSON(t, h, "POST", "/api/move", hdr, map[string]interface{}{"index": 0, "version": v.MoveVersion}, v); code != http.StatusOK {
			t.Fatalf("move: %d", code)
		}
	}
	if v.Status == StatusHumanTurn {
		t.Fatal("game did not finish")
	}
}

// readRecords parses the ratings jsonl (a missing file is zero records).
func readRecords(t *testing.T, path string) []playtest.Record {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	var recs []playtest.Record
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var rec playtest.Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad results line %q: %v", line, err)
		}
		recs = append(recs, rec)
	}
	return recs
}

func postJSON(t *testing.T, url string, body map[string]interface{}, out interface{}) int {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

// Many goroutines, each playing its own game to completion and rating it. Run
// under -race, this exercises the store map and the out-of-lock session setup in
// handleNew under contention.
func TestConcurrentDistinctSessions(t *testing.T) {
	ts, ids := testServer(t)
	var wg sync.WaitGroup
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func(gameID string) {
			defer wg.Done()
			var v View
			if code := postJSON(t, ts.URL+"/api/new", map[string]interface{}{"game": gameID, "difficulty": "random"}, &v); code != http.StatusOK {
				t.Errorf("new: status %d", code)
				return
			}
			for v.Status == StatusHumanTurn {
				postJSON(t, ts.URL+"/api/move", map[string]interface{}{"session": v.Session, "index": 0, "version": v.MoveVersion}, &v)
			}
			var ok map[string]bool
			postJSON(t, ts.URL+"/api/rate", map[string]interface{}{"session": v.Session, "rating": 3, "comment": "x"}, &ok)
		}(ids[w%len(ids)])
	}
	wg.Wait()
}

// One session, many concurrent readers (state, incl. the rules payload) racing
// against a writer playing moves. Asserts the server never returns a 5xx (a
// panic/race would surface as one) -- the real test is -race cleanliness on the
// shared WebSession fields.
func TestConcurrentSameSession(t *testing.T) {
	ts, ids := testServer(t)
	var start View
	if code := postJSON(t, ts.URL+"/api/new", map[string]interface{}{"game": ids[0], "difficulty": "random"}, &start); code != http.StatusOK {
		t.Fatalf("new: status %d", code)
	}
	sid := start.Session

	var wg sync.WaitGroup
	// readers
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				resp, err := http.Get(ts.URL + "/api/state?session=" + sid + "&rules=1")
				if err != nil {
					t.Errorf("state: %v", err)
					return
				}
				if resp.StatusCode >= 500 {
					t.Errorf("state: 5xx %d", resp.StatusCode)
				}
				resp.Body.Close()
			}
		}()
	}
	// writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		v := start
		for i := 0; i < 200; i++ {
			code := postJSON(t, ts.URL+"/api/move", map[string]interface{}{"session": sid, "index": 0, "version": v.MoveVersion}, &v)
			if code >= 500 {
				t.Errorf("move: 5xx %d", code)
				return
			}
			if v.Status != StatusHumanTurn {
				break
			}
		}
	}()
	wg.Wait()
}

// Sessions idle past sessionIdleTTL are evicted, while any state/move/rate
// touch resets the idle clock -- so maxSessions caps concurrent live games,
// not games-ever-created.
func TestIdleSessionEviction(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()
	now := time.Now()
	srv.now = func() time.Time { return now }

	var a, b View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &a); code != http.StatusOK {
		t.Fatalf("new a: %d", code)
	}
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &b); code != http.StatusOK {
		t.Fatalf("new b: %d", code)
	}

	// a is touched at +20m; b is never touched again.
	srv.now = func() time.Time { return now.Add(20 * time.Minute) }
	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": a.Session}, nil, nil); code != http.StatusOK {
		t.Fatalf("touch a: %d", code)
	}

	// At +35m: b has been idle 35m (> 30m TTL, evict); a only 15m (keep).
	srv.now = func() time.Time { return now.Add(35 * time.Minute) }
	srv.evictIdle()

	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": b.Session}, nil, nil); code != http.StatusNotFound {
		t.Errorf("b idle 35m: want 404, got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": a.Session}, nil, nil); code != http.StatusOK {
		t.Errorf("a idle 15m: want 200, got %d", code)
	}
}

// A finished game that has been rated has nothing left to serve: it ages out
// on the shorter ratedIdleTTL while an unrated session survives.
func TestRatedSessionEvictsSooner(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()
	now := time.Now()
	srv.now = func() time.Time { return now }

	var a View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &a); code != http.StatusOK {
		t.Fatalf("new a: %d", code)
	}
	hdrA := map[string]string{"X-Session-Token": a.Session}
	for i := 0; i < 100000 && a.Status == StatusHumanTurn; i++ {
		if code := doJSON(t, h, "POST", "/api/move", hdrA, map[string]interface{}{"index": 0, "version": a.MoveVersion}, &a); code != http.StatusOK {
			t.Fatalf("move: %d", code)
		}
	}
	if a.Status == StatusHumanTurn {
		t.Fatal("game did not finish")
	}
	if code := doJSON(t, h, "POST", "/api/rate", hdrA, map[string]interface{}{"rating": 3}, nil); code != http.StatusOK {
		t.Fatalf("rate: %d", code)
	}

	var b View // unrated control, same age from here on
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &b); code != http.StatusOK {
		t.Fatalf("new b: %d", code)
	}

	srv.now = func() time.Time { return now.Add(6 * time.Minute) }
	srv.evictIdle()

	if code := doJSON(t, h, "GET", "/api/state", hdrA, nil, nil); code != http.StatusNotFound {
		t.Errorf("rated+finished at 6m: want 404, got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": b.Session}, nil, nil); code != http.StatusOK {
		t.Errorf("unrated at 6m: want 200, got %d", code)
	}
}

// At capacity /api/new answers 503, and eviction frees the slot: the cap is on
// concurrent sessions, not lifetime creations (which 503'd forever after the
// 500th game until restart).
func TestCapacityIsConcurrentNotLifetime(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	srv.maxSessions = 2
	h := srv.Handler()
	now := time.Now()
	srv.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, nil); code != http.StatusOK {
			t.Fatalf("new %d: %d", i, code)
		}
	}
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, nil); code != http.StatusServiceUnavailable {
		t.Fatalf("at capacity: want 503, got %d", code)
	}

	srv.now = func() time.Time { return now.Add(time.Hour) }
	srv.evictIdle()

	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, nil); code != http.StatusOK {
		t.Errorf("after eviction: want 200, got %d", code)
	}
}

// The background janitor (started by ListenAndServe in production) evicts on
// its own ticker, no explicit evictIdle call. Knobs are set before StartJanitor
// so the goroutine sees them via the go-statement happens-before.
func TestJanitorEvictsInBackground(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	srv.idleTTL = time.Nanosecond
	h := srv.Handler()
	srv.StartJanitor(2 * time.Millisecond)
	defer srv.StopJanitor()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": v.Session}, nil, nil)
		if code == http.StatusNotFound {
			return // evicted by the janitor
		}
		if time.Now().After(deadline) {
			t.Fatal("janitor never evicted the idle session")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A session takes exactly one rating: replayed /api/rate calls get 409 and
// append nothing (no jsonl growth, no rating-dataset stuffing).
func TestRateOncePerSession(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	hdr := map[string]string{"X-Session-Token": v.Session}
	finishGame(t, h, hdr, &v) // only a finished game is rateable
	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 4, "comment": "fun"}, nil); code != http.StatusOK {
		t.Fatalf("first rate: want 200, got %d", code)
	}
	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 1, "comment": "spam"}, nil); code != http.StatusConflict {
		t.Fatalf("second rate: want 409, got %d", code)
	}
	data, err := os.ReadFile(srv.ResultsPath)
	if err != nil {
		t.Fatalf("read results: %v", err)
	}
	if n := strings.Count(string(data), "\n"); n != 1 {
		t.Errorf("results file has %d records, want 1", n)
	}
}

// A move must echo the moveVersion of the list it was chosen from; a stale
// echo (double-click racing the regenerated list) gets 409 and applies nothing.
func TestStaleMoveVersionRejected(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	if v.Status != StatusHumanTurn {
		t.Fatalf("expected human turn, got %q", v.Status)
	}
	hdr := map[string]string{"X-Session-Token": v.Session}

	if code := doJSON(t, h, "POST", "/api/move", hdr, map[string]interface{}{"index": 0, "version": v.MoveVersion + 1}, nil); code != http.StatusConflict {
		t.Fatalf("wrong version: want 409, got %d", code)
	}

	prev := v.MoveVersion
	var after View
	if code := doJSON(t, h, "POST", "/api/move", hdr, map[string]interface{}{"index": 0, "version": prev}, &after); code != http.StatusOK {
		t.Fatalf("current version: want 200, got %d", code)
	}

	// The double-click: replaying the consumed version against the regenerated
	// list must 409, not silently apply index 0 of the new list.
	if after.Status == StatusHumanTurn {
		if after.MoveVersion == prev {
			t.Fatal("moveVersion did not advance after a move")
		}
		if code := doJSON(t, h, "POST", "/api/move", hdr, map[string]interface{}{"index": 0, "version": prev}, nil); code != http.StatusConflict {
			t.Errorf("replayed version: want 409, got %d", code)
		}
	}
}

// The session token travels in the X-Session-Token header (query strings land
// in reverse-proxy access logs); the header wins, with body/query fallbacks.
func TestHeaderTokenAuth(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	hdr := map[string]string{"X-Session-Token": v.Session}

	var st View
	if code := doJSON(t, h, "GET", "/api/state", hdr, nil, &st); code != http.StatusOK {
		t.Fatalf("state via header: want 200, got %d", code)
	}
	if st.Session != v.Session {
		t.Errorf("state returned session %q, want %q", st.Session, v.Session)
	}
	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": "bogus"}, nil, nil); code != http.StatusNotFound {
		t.Errorf("bogus header token: want 404, got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state?session=bogus", hdr, nil, nil); code != http.StatusOK {
		t.Errorf("header must take precedence over query: want 200, got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state?session="+v.Session, nil, nil, nil); code != http.StatusOK {
		t.Errorf("legacy query fallback: want 200, got %d", code)
	}
	if v.Status == StatusHumanTurn {
		if code := doJSON(t, h, "POST", "/api/move", hdr, map[string]interface{}{"index": 0, "version": v.MoveVersion}, nil); code != http.StatusOK {
			t.Errorf("move via header token: want 200, got %d", code)
		}
	}
}

// Any client-supplied seed is ignored: the seed determines the entire shuffle,
// so honoring it (or a predictable timestamp default) hands the client every
// hidden hand. The server always draws the seed from crypto/rand.
func TestClientSeedIgnored(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random", "seed": 42}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	srv.mu.RLock()
	ws := srv.store[v.Session]
	srv.mu.RUnlock()
	if ws == nil {
		t.Fatal("session not stored")
	}
	if ws.Seed == 42 {
		t.Error("client-supplied seed was honored; seeds must be server-random")
	}
}

// A game can only be rated once it has ended. A mid-game rating used to be
// logged with winner:"none" (indistinguishable from a turn-limit game) and
// spent the session's one rating; now it is refused with 409, logs nothing,
// and leaves the rating slot free for the real end-of-game rating.
func TestRateRefusedBeforeGameEnds(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	if v.Status != StatusHumanTurn {
		t.Fatalf("expected a game in progress, got %q", v.Status)
	}
	hdr := map[string]string{"X-Session-Token": v.Session}

	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 1}, nil); code != http.StatusConflict {
		t.Fatalf("mid-game rate: want 409, got %d", code)
	}
	if recs := readRecords(t, srv.ResultsPath); len(recs) != 0 {
		t.Fatalf("mid-game rate logged %d record(s), want 0: %+v", len(recs), recs)
	}

	// The refusal must not have consumed the one-rating slot.
	finishGame(t, h, hdr, &v)
	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 4}, nil); code != http.StatusOK {
		t.Fatalf("end-of-game rate after a refused mid-game rate: want 200, got %d", code)
	}
	recs := readRecords(t, srv.ResultsPath)
	if len(recs) != 1 {
		t.Fatalf("results file has %d records, want 1", len(recs))
	}
	if recs[0].Rating == nil || *recs[0].Rating != 4 {
		t.Errorf("logged rating = %v, want 4", recs[0].Rating)
	}
	if recs[0].Turns != v.Turn || recs[0].Turns == 0 {
		t.Errorf("logged turns = %d, want the finished game's %d (non-zero)", recs[0].Turns, v.Turn)
	}
}

// A stuck game is also over (nothing left to play), so it stays rateable and
// the record says so.
func TestRateAllowedWhenStuck(t *testing.T) {
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()

	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	srv.mu.RLock()
	ws := srv.store[v.Session]
	srv.mu.RUnlock()
	ws.mu.Lock()
	ws.status = StatusStuck
	ws.legalMoves = nil
	ws.mu.Unlock()

	hdr := map[string]string{"X-Session-Token": v.Session}
	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 2}, nil); code != http.StatusOK {
		t.Fatalf("rate a stuck game: want 200, got %d", code)
	}
	recs := readRecords(t, srv.ResultsPath)
	if len(recs) != 1 {
		t.Fatalf("results file has %d records, want 1", len(recs))
	}
	if !recs[0].Stuck || recs[0].Winner != "stuck" {
		t.Errorf("stuck record = %+v, want stuck:true winner:\"stuck\"", recs[0])
	}
}

// gatedAI wraps a real AI and, once armed, parks inside SelectMove until
// released -- a stand-in for a slow MCTS decision. The session lock is held for
// the whole call (submitMove -> advance -> SelectMove), which is exactly the
// window the reaper must not wait on while holding the server-wide lock.
type gatedAI struct {
	inner   sim.AIPlayer
	armed   atomic.Bool
	once    sync.Once
	entered chan struct{} // closed when the first armed call parks
	release chan struct{} // close to let the parked call (and all later ones) through
}

func (a *gatedAI) SelectMove(moves []sim.Move, st *sim.GameState, rng *rand.Rand) sim.Move {
	if a.armed.Load() {
		a.once.Do(func() { close(a.entered) })
		<-a.release
	}
	return a.inner.SelectMove(moves, st, rng)
}

// The reaper used to take the server-wide lock and then wait on each session's
// own lock, so ONE session busy computing an AI move froze every other
// session's requests (their lookup needs the server lock) for as long as that
// move took. A busy session must cost the others nothing -- and the sweep must
// still evict what is idle and keep what is live.
func TestReaperDoesNotStallOtherSessions(t *testing.T) {
	g := firstShedding(t)
	srv := serverWith(t, g)
	h := srv.Handler()
	t0 := time.Now()
	srv.now = func() time.Time { return t0 }

	// idle: created at t0 and never touched again.
	var idle View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &idle); code != http.StatusOK {
		t.Fatalf("new idle: %d", code)
	}

	// Everything else happens 35 minutes later: past idle's 30m TTL. The clock
	// is not reassigned again, so the goroutines below read it race-free.
	later := t0.Add(35 * time.Minute)
	srv.now = func() time.Time { return later }

	var other View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &other); code != http.StatusOK {
		t.Fatalf("new other: %d", code)
	}
	otherHdr := map[string]string{"X-Session-Token": other.Session}

	// busy: a live session whose next AI decision parks while holding its lock.
	ai := &gatedAI{inner: &sim.RandomAI{}, entered: make(chan struct{}), release: make(chan struct{})}
	busy := NewWebSession("busy", g, fitness.GetRunner(g), ai, 99, "random", "seed.json")
	busy.touch(later)
	srv.mu.Lock()
	srv.store[busy.ID] = busy
	srv.mu.Unlock()
	busyHdr := map[string]string{"X-Session-Token": busy.ID}
	var bv View
	if code := doJSON(t, h, "GET", "/api/state", busyHdr, nil, &bv); code != http.StatusOK {
		t.Fatalf("state busy: %d", code)
	}
	if bv.Status != StatusHumanTurn {
		t.Fatalf("busy session: expected a human turn, got %q", bv.Status)
	}

	ai.armed.Store(true)
	release := sync.OnceFunc(func() { close(ai.release) })
	defer release() // never leave a goroutine parked, whatever fails below

	// Play the human seat until the AI is consulted; that request then sits in
	// SelectMove holding busy's lock.
	moveDone := make(chan struct{})
	go func() {
		defer close(moveDone)
		v := bv
		for v.Status == StatusHumanTurn {
			if code := doJSON(t, h, "POST", "/api/move", busyHdr, map[string]interface{}{"index": 0, "version": v.MoveVersion}, &v); code != http.StatusOK {
				t.Errorf("busy move: %d", code)
				return
			}
			select {
			case <-ai.entered:
				return // that was the gated move, now released
			default:
			}
		}
	}()
	select {
	case <-ai.entered:
	case <-moveDone:
		t.Fatal("busy game ended without ever consulting the AI")
	case <-time.After(10 * time.Second):
		t.Fatal("the AI was never consulted")
	}

	evictDone := make(chan struct{})
	go func() {
		defer close(evictDone)
		srv.evictIdle()
	}()

	// While the sweep runs against a locked session, an unrelated session must
	// keep answering. Probe for a short window so the reaper has certainly
	// reached the busy session; the stall limit is generous so a loaded machine
	// cannot fail a correct server.
	const stallLimit = 5 * time.Second
	var stalled chan int
	for until := time.Now().Add(100 * time.Millisecond); stalled == nil && time.Now().Before(until); time.Sleep(time.Millisecond) {
		done := make(chan int, 1)
		go func() { done <- doJSON(t, h, "GET", "/api/state", otherHdr, nil, nil) }()
		select {
		case code := <-done:
			if code != http.StatusOK {
				t.Fatalf("state other: %d", code)
			}
		case <-time.After(stallLimit):
			t.Errorf("/api/state on an unrelated session stalled > %v behind the reaper waiting on a busy session's lock", stallLimit)
			stalled = done
		}
	}

	release()
	for name, ch := range map[string]chan struct{}{"busy move": moveDone, "evictIdle": evictDone} {
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("%s never finished after the AI was released", name)
		}
	}
	if stalled != nil {
		<-stalled // let the stalled probe drain before the test returns
	}

	// The sweep still did its job around the busy session.
	if code := doJSON(t, h, "GET", "/api/state", map[string]string{"X-Session-Token": idle.Session}, nil, nil); code != http.StatusNotFound {
		t.Errorf("idle 35m: want 404 (evicted), got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state", otherHdr, nil, nil); code != http.StatusOK {
		t.Errorf("recently created session: want 200 (kept), got %d", code)
	}
	if code := doJSON(t, h, "GET", "/api/state", busyHdr, nil, nil); code != http.StatusOK {
		t.Errorf("busy session: want 200 (kept), got %d", code)
	}
}

// finishedSession starts a game on a fresh single-seed server and plays it to
// the end, returning everything a rating test needs.
func finishedSession(t *testing.T) (*Server, http.Handler, map[string]string) {
	t.Helper()
	srv := serverWith(t, firstShedding(t))
	h := srv.Handler()
	var v View
	if code := doJSON(t, h, "POST", "/api/new", nil, map[string]interface{}{"difficulty": "random"}, &v); code != http.StatusOK {
		t.Fatalf("new: %d", code)
	}
	hdr := map[string]string{"X-Session-Token": v.Session}
	finishGame(t, h, hdr, &v)
	return srv, h, hdr
}

// A rating outside 1-5 that is not the explicit skip (0) is a client error. It
// used to be saved as rating:null with a 200 -- silently turning "9" into a
// skip and spending the session's one rating. Now: 400, nothing logged, and
// the slot is still free for a valid rating.
func TestRateOutOfRangeRejected(t *testing.T) {
	srv, h, hdr := finishedSession(t)

	for _, bad := range []interface{}{9, -3, 6, -1, 3.5} {
		if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": bad}, nil); code != http.StatusBadRequest {
			t.Errorf("rating %v: want 400, got %d", bad, code)
		}
	}
	if recs := readRecords(t, srv.ResultsPath); len(recs) != 0 {
		t.Fatalf("rejected ratings logged %d record(s), want 0: %+v", len(recs), recs)
	}

	// None of the rejections consumed the one-rating slot.
	if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": 5}, nil); code != http.StatusOK {
		t.Fatalf("valid rating after rejected ones: want 200, got %d", code)
	}
	recs := readRecords(t, srv.ResultsPath)
	if len(recs) != 1 || recs[0].Rating == nil || *recs[0].Rating != 5 {
		t.Fatalf("records = %+v, want exactly one with rating 5", recs)
	}
}

// Skip stays a first-class answer: rating 0 (what the page sends when no star
// is picked) and an omitted rating field both record rating:null.
func TestRateSkipRecordsNullRating(t *testing.T) {
	for name, body := range map[string]map[string]interface{}{
		"explicit zero": {"rating": 0, "comment": "skipped"},
		"omitted field": {"comment": "skipped"},
	} {
		t.Run(name, func(t *testing.T) {
			srv, h, hdr := finishedSession(t)
			if code := doJSON(t, h, "POST", "/api/rate", hdr, body, nil); code != http.StatusOK {
				t.Fatalf("skip: want 200, got %d", code)
			}
			recs := readRecords(t, srv.ResultsPath)
			if len(recs) != 1 {
				t.Fatalf("results file has %d records, want 1", len(recs))
			}
			if recs[0].Rating != nil {
				t.Errorf("skip logged rating %d, want null", *recs[0].Rating)
			}
			if recs[0].Comment != "skipped" {
				t.Errorf("skip dropped the comment: %+v", recs[0])
			}
		})
	}
}

// The ends of the 1-5 scale are valid ratings and are logged as given.
func TestRateBoundsAccepted(t *testing.T) {
	for _, rating := range []int{1, 5} {
		srv, h, hdr := finishedSession(t)
		if code := doJSON(t, h, "POST", "/api/rate", hdr, map[string]interface{}{"rating": rating}, nil); code != http.StatusOK {
			t.Fatalf("rating %d: want 200, got %d", rating, code)
		}
		recs := readRecords(t, srv.ResultsPath)
		if len(recs) != 1 || recs[0].Rating == nil || *recs[0].Rating != rating {
			t.Errorf("rating %d: records = %+v, want exactly one with that rating", rating, recs)
		}
	}
}

// indexPage fetches the embedded UI through the real handler.
func indexPage(t *testing.T) string {
	t.Helper()
	srv := serverWith(t, firstShedding(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: %d", rec.Code)
	}
	return rec.Body.String()
}

// The rating panel is the only place the page calls /api/rate from, and it is
// revealed only for a terminal status. A 409 while the page does not believe
// the game is over is the not-finished refusal, which must not be reported as
// "already recorded" (that would also disable the submit button for good).
func TestIndexRatingOnlyOfferedAtGameEnd(t *testing.T) {
	page := indexPage(t)
	for _, want := range []string{
		`id="overPanel" hidden`, // the rating panel starts hidden
		`gameOver = v.status === "game_over" || v.status === "stuck";`,
		`$("overPanel").hidden = !gameOver;`,
		`if (e.status === 409 && !gameOver)`, // not-finished handled apart from already-rated
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index.html is missing %q", want)
		}
	}
}
