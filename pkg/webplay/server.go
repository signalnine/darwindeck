package webplay

import (
	"crypto/rand"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/darwindeck/darwindeck/pkg/fitness"
	"github.com/darwindeck/darwindeck/pkg/genome"
	"github.com/darwindeck/darwindeck/pkg/output"
	"github.com/darwindeck/darwindeck/pkg/playtest"
	"github.com/darwindeck/darwindeck/pkg/sim"
)

//go:embed static/index.html
var staticFS embed.FS

const (
	defaultMaxSessions = 500     // cap on CONCURRENT live sessions (idle ones are janitor-evicted)
	maxBody            = 4 << 10 // 4 KiB: every request body here is tiny JSON

	// Janitor TTLs. Sessions idle past their TTL are evicted, so maxSessions
	// bounds concurrent live games, not games-ever-created (which would turn the
	// cap into a permanent 503 after the 500th game until restart).
	sessionIdleTTL = 30 * time.Minute // untouched sessions are abandoned
	ratedIdleTTL   = 5 * time.Minute  // finished + rated: nothing left to do
	janitorPeriod  = time.Minute      // eviction sweep cadence
)

// Game is one playable genome registered with the server. The client only ever
// names a Game by ID (assigned here, "g0".."gN"); it never sends a file path,
// so no client input reaches the filesystem.
type Game struct {
	ID       string
	Title    string
	Skeleton string
	Genome   *genome.Genome
	Path     string // source file, recorded in the ratings log; server-internal
}

// Server serves the registered games over HTTP.
type Server struct {
	games []Game
	byID  map[string]Game
	mu    sync.RWMutex
	store map[string]*WebSession
	// reserved counts slots claimed by in-flight handleNew constructions. The
	// capacity check charges store+reserved BEFORE the expensive session build,
	// so a full store rejects with a cheap 503 instead of paying game setup +
	// AI auto-play + rulebook rendering per request (a CPU DoS).
	reserved int

	// ResultsPath is where /api/rate appends rating records. Defaults to the
	// shared playtest.ResultsFile so web and CLI playtests pool into one file;
	// tests point it at a temp file to avoid polluting the repo.
	ResultsPath string

	// Eviction knobs, defaulted from the consts above; tests shrink the TTLs or
	// inject a clock instead of sleeping.
	maxSessions int
	idleTTL     time.Duration
	ratedTTL    time.Duration
	now         func() time.Time

	janitorStop chan struct{}
}

// NewServer builds a server over a registry of preloaded, validated genomes.
func NewServer(games []Game) *Server {
	byID := make(map[string]Game, len(games))
	for _, g := range games {
		byID[g.ID] = g
	}
	return &Server{
		games:       games,
		byID:        byID,
		store:       make(map[string]*WebSession),
		ResultsPath: playtest.ResultsFile,
		maxSessions: defaultMaxSessions,
		idleTTL:     sessionIdleTTL,
		ratedTTL:    ratedIdleTTL,
		now:         time.Now,
	}
}

// RegisterGame builds a Game entry from a validated genome, assigning a stable
// client-facing ID. Index keeps IDs unique even when genome.ID is blank or
// duplicated.
func RegisterGame(index int, g *genome.Genome, path string) Game {
	return Game{
		ID:       fmt.Sprintf("g%d", index),
		Title:    gameTitle(g),
		Skeleton: g.Skeleton.String(),
		Genome:   g,
		Path:     path,
	}
}

// Handler returns the mux with all routes wired.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/games", s.handleGames)
	mux.HandleFunc("/api/new", s.handleNew)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/move", s.handleMove)
	mux.HandleFunc("/api/rate", s.handleRate)
	return mux
}

// ListenAndServe starts the server with conservative timeouts.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second, // accommodates a chain of MCTS AI decisions
		IdleTimeout:       120 * time.Second,
	}
	s.StartJanitor(janitorPeriod)
	defer s.StopJanitor()
	return srv.ListenAndServe()
}

// StartJanitor launches the background loop that evicts idle sessions, freeing
// capacity so maxSessions caps concurrent games rather than lifetime games.
// Call StopJanitor to shut it down (tests; server teardown).
func (s *Server) StartJanitor(period time.Duration) {
	stop := make(chan struct{})
	s.janitorStop = stop
	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.evictIdle()
			case <-stop:
				return
			}
		}
	}()
}

// StopJanitor stops the eviction loop started by StartJanitor.
func (s *Server) StopJanitor() {
	if s.janitorStop != nil {
		close(s.janitorStop)
		s.janitorStop = nil
	}
}

// evictIdle deletes every session idle past its TTL: ratedTTL once a finished
// game has been rated (the session's whole point is spent), idleTTL otherwise.
// Lock order is s.mu then ws.mu, the same direction every handler uses.
func (s *Server) evictIdle() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ws := range s.store {
		ws.mu.Lock()
		last := ws.lastActive
		spent := ws.rated && ws.status != StatusHumanTurn
		ws.mu.Unlock()
		ttl := s.idleTTL
		if spent {
			ttl = s.ratedTTL
		}
		if now.Sub(last) > ttl {
			delete(s.store, id)
		}
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "missing UI", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// gameListItem is the picker payload (no genome internals leak).
type gameListItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Skeleton    string `json:"skeleton"`
	Players     int    `json:"players"`
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	items := make([]gameListItem, 0, len(s.games))
	for _, g := range s.games {
		items = append(items, gameListItem{
			ID:          g.ID,
			Title:       g.Title,
			Description: g.Genome.Description,
			Skeleton:    g.Skeleton,
			Players:     g.Genome.Players,
		})
	}
	writeJSON(w, items)
}

// newReq deliberately has no seed field: the seed is always derived server-side
// from crypto/rand (see handleNew). A client-chosen or predictable (timestamp)
// seed lets the client reconstruct hidden state -- vying hole cards become
// known or brute-forceable. Any "seed" the client sends is ignored.
type newReq struct {
	Game       string `json:"game"`
	Difficulty string `json:"difficulty"`
}

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req newReq
	if !decodeBody(w, r, &req) {
		return
	}

	game, ok := s.byID[req.Game]
	if !ok {
		// Single-game servers: default to the only game when none is named.
		if req.Game == "" && len(s.games) > 0 {
			game = s.games[0]
		} else {
			http.Error(w, "unknown game", http.StatusBadRequest)
			return
		}
	}

	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = "greedy"
	}
	runner := fitness.GetRunner(game.Genome)
	if runner == nil {
		http.Error(w, "no runner for skeleton", http.StatusInternalServerError)
		return
	}
	ai, err := aiFor(difficulty, game.Genome, runner)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	seed, err := randomSeed()
	if err != nil {
		http.Error(w, "seed error", http.StatusInternalServerError)
		return
	}
	id, err := newToken()
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	// Reserve a capacity slot BEFORE the expensive construction below: without
	// the reservation a full store would still pay full setup cost per request
	// before its 503. The deferred release covers every non-commit exit.
	s.mu.Lock()
	if len(s.store)+s.reserved >= s.maxSessions {
		s.mu.Unlock()
		http.Error(w, "too many active sessions", http.StatusServiceUnavailable)
		return
	}
	s.reserved++
	s.mu.Unlock()
	committed := false
	defer func() {
		if !committed {
			s.mu.Lock()
			s.reserved--
			s.mu.Unlock()
		}
	}()

	// Build the session and render its rulebook OUTSIDE s.mu: NewWebSession runs
	// the game setup + AI auto-play to the first human decision (which can be slow
	// under MCTS), and GenerateRulebook builds a sizeable string. Holding the
	// store lock across either would block every other session's lookups. ws is
	// fully initialized (incl. ws.rules) before it is published to the store, so
	// the s.mu insert is the only happens-before edge a reader needs.
	ws := NewWebSession(id, game.Genome, runner, ai, seed, difficulty, game.Path)
	ws.rules = output.GenerateRulebook(game.Genome)
	ws.lastActive = s.now() // pre-publish: no other goroutine can hold ws.mu yet

	s.mu.Lock()
	s.reserved--
	s.store[id] = ws
	committed = true
	s.mu.Unlock()

	ws.mu.Lock()
	v := ws.view(true)
	ws.mu.Unlock()
	writeJSON(w, v)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	ws := s.lookup(w, sessionToken(r, ""))
	if ws == nil {
		return
	}
	includeRules := r.URL.Query().Get("rules") == "1"
	ws.mu.Lock()
	v := ws.view(includeRules)
	ws.mu.Unlock()
	writeJSON(w, v)
}

type moveReq struct {
	Session string `json:"session"` // legacy fallback; X-Session-Token preferred
	Index   int    `json:"index"`
	Version int    `json:"version"` // moveVersion echo; stale = 409
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req moveReq
	if !decodeBody(w, r, &req) {
		return
	}
	ws := s.lookup(w, sessionToken(r, req.Session))
	if ws == nil {
		return
	}
	if err := ws.submitMove(req.Index, req.Version); err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, errStaleMove) {
			code = http.StatusConflict
		}
		http.Error(w, err.Error(), code)
		return
	}
	ws.mu.Lock()
	v := ws.view(false)
	ws.mu.Unlock()
	writeJSON(w, v)
}

type rateReq struct {
	Session string `json:"session"` // legacy fallback; X-Session-Token preferred
	Rating  int    `json:"rating"`  // 1-5; 0 = skip
	Comment string `json:"comment"`
}

func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req rateReq
	if !decodeBody(w, r, &req) {
		return
	}
	ws := s.lookup(w, sessionToken(r, req.Session))
	if ws == nil {
		return
	}

	var rating *int
	if req.Rating >= 1 && req.Rating <= 5 {
		v := req.Rating
		rating = &v
	}

	// One rating per session: the flag flips under ws.mu before the write, so a
	// concurrent duplicate loses here (409) instead of double-appending to the
	// jsonl (unbounded growth / rating-dataset poisoning from replayed calls).
	ws.mu.Lock()
	if ws.rated {
		ws.mu.Unlock()
		http.Error(w, "session already rated", http.StatusConflict)
		return
	}
	ws.rated = true
	rec := playtest.Record{
		Timestamp:  time.Now().Format(time.RFC3339),
		GenomeID:   ws.Genome.ID,
		GenomePath: ws.GenomePath,
		Difficulty: ws.Difficulty,
		Seed:       ws.Seed,
		Winner:     ws.ratingWinnerLabel(),
		Turns:      ws.state.Turn,
		Rating:     rating,
		Comment:    req.Comment,
		Stuck:      ws.status == StatusStuck,
	}
	ws.mu.Unlock()

	if err := playtest.AppendRecord(s.ResultsPath, rec); err != nil {
		// Nothing was appended; let the client retry rather than losing the rating.
		ws.mu.Lock()
		ws.rated = false
		ws.mu.Unlock()
		http.Error(w, "failed to record rating", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// ratingWinnerLabel maps the session's terminal status to the
// playtest_results.jsonl winner vocabulary. Caller holds ws.mu.
func (ws *WebSession) ratingWinnerLabel() string {
	switch {
	case ws.status == StatusStuck:
		return "stuck"
	case ws.status != StatusGameOver:
		return "none"
	case ws.winner < 0:
		return "none"
	case ws.winner == HumanSeat:
		return "human"
	default:
		return "ai"
	}
}

func (s *Server) lookup(w http.ResponseWriter, id string) *WebSession {
	if id == "" {
		http.Error(w, "missing session", http.StatusBadRequest)
		return nil
	}
	s.mu.RLock()
	ws := s.store[id]
	s.mu.RUnlock()
	if ws == nil {
		http.Error(w, "unknown session", http.StatusNotFound)
		return nil
	}
	ws.touch(s.now()) // every authenticated request resets the idle-eviction clock
	return ws
}

// sessionToken extracts the client's session token. The X-Session-Token header
// is preferred -- query strings land verbatim in reverse-proxy access logs
// (Caddy), leaking a live credential. The JSON-body field and ?session= query
// parameter remain as fallbacks for older clients.
func sessionToken(r *http.Request, bodyToken string) string {
	if h := r.Header.Get("X-Session-Token"); h != "" {
		return h
	}
	if bodyToken != "" {
		return bodyToken
	}
	return r.URL.Query().Get("session")
}

func aiFor(difficulty string, g *genome.Genome, runner sim.GenericRunner) (sim.AIPlayer, error) {
	switch difficulty {
	case "random":
		return &sim.RandomAI{}, nil
	case "greedy":
		return fitness.GetGreedyAI(g), nil
	case "mcts":
		return playtest.NewMCTSAI(g, runner), nil
	default:
		return nil, fmt.Errorf("unknown difficulty %q (use random, greedy, or mcts)", difficulty)
	}
}

func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// randomSeed draws the game seed from crypto/rand. The seed fully determines
// the shuffle, so it must be unpredictable to the client (see newReq).
func randomSeed() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
