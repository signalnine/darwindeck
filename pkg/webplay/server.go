package webplay

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
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
	maxSessions = 500     // bound in-memory sessions (a DoS guard when -host exposes the server)
	maxBody     = 4 << 10 // 4 KiB: every request body here is tiny JSON
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

	// ResultsPath is where /api/rate appends rating records. Defaults to the
	// shared playtest.ResultsFile so web and CLI playtests pool into one file;
	// tests point it at a temp file to avoid polluting the repo.
	ResultsPath string
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
	return srv.ListenAndServe()
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
	ID       string `json:"id"`
	Title    string `json:"title"`
	Skeleton string `json:"skeleton"`
	Players  int    `json:"players"`
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	items := make([]gameListItem, 0, len(s.games))
	for _, g := range s.games {
		items = append(items, gameListItem{
			ID:       g.ID,
			Title:    g.Title,
			Skeleton: g.Skeleton,
			Players:  g.Genome.Players,
		})
	}
	writeJSON(w, items)
}

type newReq struct {
	Game       string `json:"game"`
	Difficulty string `json:"difficulty"`
	Seed       uint64 `json:"seed"`
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

	seed := req.Seed
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}

	// Build the session and render its rulebook OUTSIDE s.mu: NewWebSession runs
	// the game setup + AI auto-play to the first human decision (which can be slow
	// under MCTS), and GenerateRulebook builds a sizeable string. Holding the
	// store lock across either would block every other session's lookups. ws is
	// fully initialized (incl. ws.rules) before it is published to the store, so
	// the s.mu insert is the only happens-before edge a reader needs.
	id, err := newToken()
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	ws := NewWebSession(id, game.Genome, runner, ai, seed, difficulty, game.Path)
	ws.rules = output.GenerateRulebook(game.Genome)

	s.mu.Lock()
	if len(s.store) >= maxSessions {
		s.mu.Unlock()
		http.Error(w, "too many active sessions", http.StatusServiceUnavailable)
		return
	}
	s.store[id] = ws
	s.mu.Unlock()

	ws.mu.Lock()
	v := ws.view(true)
	ws.mu.Unlock()
	writeJSON(w, v)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	ws := s.lookup(w, r.URL.Query().Get("session"))
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
	Session string `json:"session"`
	Index   int    `json:"index"`
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
	ws := s.lookup(w, req.Session)
	if ws == nil {
		return
	}
	if err := ws.submitMove(req.Index); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ws.mu.Lock()
	v := ws.view(false)
	ws.mu.Unlock()
	writeJSON(w, v)
}

type rateReq struct {
	Session string `json:"session"`
	Rating  int    `json:"rating"` // 1-5; 0 = skip
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
	ws := s.lookup(w, req.Session)
	if ws == nil {
		return
	}

	var rating *int
	if req.Rating >= 1 && req.Rating <= 5 {
		v := req.Rating
		rating = &v
	}

	ws.mu.Lock()
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
	return ws
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
