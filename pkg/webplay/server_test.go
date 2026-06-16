package webplay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

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
				postJSON(t, ts.URL+"/api/move", map[string]interface{}{"session": v.Session, "index": 0}, &v)
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
		var v View
		for i := 0; i < 200; i++ {
			code := postJSON(t, ts.URL+"/api/move", map[string]interface{}{"session": sid, "index": 0}, &v)
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
