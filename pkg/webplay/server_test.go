package webplay

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/seeds"
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
