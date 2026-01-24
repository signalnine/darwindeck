// Package main provides a Go worker binary for isolated simulation.
// It reads JSON commands from stdin and writes JSON responses to stdout.
// This provides crash isolation - buggy genomes crash the worker, not the web server.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/signalnine/darwindeck/gosim/engine"
	"github.com/signalnine/darwindeck/gosim/simulation"
)

// Command represents an incoming JSON command from Python.
type Command struct {
	Action    string          `json:"action"`
	Genome    json.RawMessage `json:"genome,omitempty"`
	State     json.RawMessage `json:"state,omitempty"`
	MoveIndex int             `json:"move_index,omitempty"`
	AIType    string          `json:"ai_type,omitempty"`
	Seed      int64           `json:"seed,omitempty"`
}

// Response represents the JSON response sent to Python.
type Response struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	State   json.RawMessage `json:"state,omitempty"`
	Moves   []MoveInfo      `json:"moves,omitempty"`
	Winner  int             `json:"winner,omitempty"`
	AIMove  *MoveInfo       `json:"ai_move,omitempty"`
}

// MoveInfo describes a legal move for the human player.
type MoveInfo struct {
	Index     int    `json:"index"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	CardIndex int    `json:"card_index"` // Index into player's hand, -1 if not card-specific
}

// SerializedState holds game state in a JSON-friendly format.
// We serialize state to JSON directly rather than bytecode for easier debugging.
type SerializedState struct {
	Players       []SerializedPlayer `json:"players"`
	Deck          []SerializedCard   `json:"deck"`
	Discard       []SerializedCard   `json:"discard"`
	Tableau       [][]SerializedCard `json:"tableau"`
	CurrentPlayer int                `json:"current_player"`
	TurnNumber    int                `json:"turn_number"`
	WinnerID      int                `json:"winner_id"`
	NumPlayers    int                `json:"num_players"`
	// Betting state
	Pot             int64 `json:"pot"`
	CurrentBet      int64 `json:"current_bet"`
	BettingComplete bool  `json:"betting_complete"`
	// Trick-taking state
	CurrentTrick []SerializedTrickCard `json:"current_trick,omitempty"`
	TrickLeader  int                   `json:"trick_leader"`
	TricksWon    []int                 `json:"tricks_won,omitempty"`
	HeartsBroken bool                  `json:"hearts_broken"`
	// Tableau mode
	TableauMode       int `json:"tableau_mode"`
	SequenceDirection int `json:"sequence_direction"`
}

// SerializedPlayer holds player state in JSON format.
type SerializedPlayer struct {
	Hand       []SerializedCard `json:"hand"`
	Score      int              `json:"score"`
	Active     bool             `json:"active"`
	Chips      int64            `json:"chips"`
	CurrentBet int64            `json:"current_bet"`
	HasFolded  bool             `json:"has_folded"`
	IsAllIn    bool             `json:"is_all_in"`
}

// SerializedCard holds a card in JSON format.
type SerializedCard struct {
	Rank int `json:"rank"` // 0-12 (2-A)
	Suit int `json:"suit"` // 0-3 (H,D,C,S)
}

// SerializedTrickCard holds a card played to the current trick.
type SerializedTrickCard struct {
	PlayerID int            `json:"player_id"`
	Card     SerializedCard `json:"card"`
}

// Global state for the current game session
var (
	currentGenome *engine.Genome
	currentState  *engine.GameState
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size for large states/genomes
	buf := make([]byte, 1024*1024) // 1MB
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var cmd Command
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			writeError(fmt.Sprintf("invalid JSON: %v", err))
			continue
		}

		resp := handleCommand(&cmd)
		writeResponse(resp)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}
}

func handleCommand(cmd *Command) *Response {
	switch cmd.Action {
	case "ping":
		return handlePing()
	case "start_game":
		return handleStartGame(cmd)
	case "apply_move":
		return handleApplyMove(cmd)
	case "validate_genome":
		return handleValidateGenome(cmd)
	case "get_ai_move":
		return handleGetAIMove(cmd)
	default:
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", cmd.Action),
		}
	}
}

// handlePing is a health check that returns success.
func handlePing() *Response {
	return &Response{Success: true}
}

// handleStartGame initializes a new game from genome bytecode.
func handleStartGame(cmd *Command) *Response {
	// Decode genome from base64
	var genomeB64 string
	if err := json.Unmarshal(cmd.Genome, &genomeB64); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("invalid genome field: %v", err),
		}
	}

	bytecode, err := base64.StdEncoding.DecodeString(genomeB64)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("invalid base64 genome: %v", err),
		}
	}

	// Parse genome from bytecode
	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to parse genome: %v", err),
		}
	}
	currentGenome = genome

	// Initialize game state
	state := engine.GetState()

	// Setup deck
	setupDeck(state, uint64(cmd.Seed))

	// Read setup from genome
	cardsPerPlayer := 26 // Default for War
	initialDiscardCount := 0
	startingChips := 0

	if genome.Header.SetupOffset > 0 && genome.Header.SetupOffset+12 <= int32(len(genome.Bytecode)) {
		setupOffset := genome.Header.SetupOffset
		cardsPerPlayer = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset : setupOffset+4])))
		initialDiscardCount = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+4 : setupOffset+8])))
		startingChips = int(int32(binary.BigEndian.Uint32(genome.Bytecode[setupOffset+8 : setupOffset+12])))
	}

	// Number of players
	numPlayers := int(genome.Header.PlayerCount)
	if numPlayers == 0 || numPlayers > 4 {
		numPlayers = 2
	}

	state.NumPlayers = uint8(numPlayers)
	state.CardsPerPlayer = cardsPerPlayer
	state.TableauMode = genome.Header.TableauMode
	state.SequenceDirection = genome.Header.SequenceDirection

	// Initialize teams if configured
	if genome.Header.TeamMode && genome.Header.TeamCount > 0 && genome.Header.TeamDataOffset > 0 {
		teamDataOffset := genome.Header.TeamDataOffset
		if teamDataOffset < len(genome.Bytecode) {
			teams := engine.ParseTeams(genome.Bytecode[teamDataOffset:])
			state.InitializeTeams(teams)
		}
	}

	// Deal cards to each player
	for i := 0; i < cardsPerPlayer; i++ {
		for p := 0; p < numPlayers; p++ {
			state.DrawCard(uint8(p), engine.LocationDeck)
		}
	}

	// Deal initial cards to discard/tableau
	if initialDiscardCount > 0 && len(state.Deck) >= initialDiscardCount {
		if state.TableauMode != 0 && len(state.Tableau) == 0 {
			state.Tableau = make([][]engine.Card, 1)
			state.Tableau[0] = make([]engine.Card, 0, initialDiscardCount)
		}
		for i := 0; i < initialDiscardCount; i++ {
			if len(state.Deck) > 0 {
				card := state.Deck[len(state.Deck)-1]
				state.Deck = state.Deck[:len(state.Deck)-1]
				if state.TableauMode != 0 {
					state.Tableau[0] = append(state.Tableau[0], card)
				} else {
					state.Discard = append(state.Discard, card)
				}
			}
		}
	}

	// Initialize chips
	if startingChips > 0 {
		state.InitializeChips(startingChips)
	}

	currentState = state

	// Generate initial legal moves
	moves := engine.GenerateLegalMoves(state, genome)
	moveInfos := convertMoves(moves, state, genome)

	// Serialize state
	stateJSON, err := json.Marshal(serializeState(state))
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to serialize state: %v", err),
		}
	}

	// Check for immediate winner
	winner := engine.CheckWinConditions(state, genome)

	return &Response{
		Success: true,
		State:   stateJSON,
		Moves:   moveInfos,
		Winner:  int(winner),
	}
}

// handleApplyMove applies a move to the current game state.
func handleApplyMove(cmd *Command) *Response {
	if currentGenome == nil || currentState == nil {
		return &Response{
			Success: false,
			Error:   "no game in progress - call start_game first",
		}
	}

	// Optionally load state from command (for stateless operation)
	if cmd.State != nil && len(cmd.State) > 0 {
		var serialized SerializedState
		if err := json.Unmarshal(cmd.State, &serialized); err != nil {
			return &Response{
				Success: false,
				Error:   fmt.Sprintf("invalid state: %v", err),
			}
		}
		deserializeState(&serialized, currentState)
	}

	// Generate legal moves and find the requested one
	moves := engine.GenerateLegalMoves(currentState, currentGenome)
	if cmd.MoveIndex < 0 || cmd.MoveIndex >= len(moves) {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("invalid move index %d (have %d moves)", cmd.MoveIndex, len(moves)),
		}
	}

	// Apply the move
	move := &moves[cmd.MoveIndex]
	engine.ApplyMove(currentState, move, currentGenome)

	// Check for winner
	winner := engine.CheckWinConditions(currentState, currentGenome)

	// Generate new legal moves
	newMoves := engine.GenerateLegalMoves(currentState, currentGenome)
	moveInfos := convertMoves(newMoves, currentState, currentGenome)

	// Serialize state
	stateJSON, err := json.Marshal(serializeState(currentState))
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to serialize state: %v", err),
		}
	}

	return &Response{
		Success: true,
		State:   stateJSON,
		Moves:   moveInfos,
		Winner:  int(winner),
	}
}

// handleGetAIMove selects a move using the specified AI type.
func handleGetAIMove(cmd *Command) *Response {
	if currentGenome == nil || currentState == nil {
		return &Response{
			Success: false,
			Error:   "no game in progress - call start_game first",
		}
	}

	// Optionally load state from command
	if cmd.State != nil && len(cmd.State) > 0 {
		var serialized SerializedState
		if err := json.Unmarshal(cmd.State, &serialized); err != nil {
			return &Response{
				Success: false,
				Error:   fmt.Sprintf("invalid state: %v", err),
			}
		}
		deserializeState(&serialized, currentState)
	}

	// Generate legal moves
	moves := engine.GenerateLegalMoves(currentState, currentGenome)
	if len(moves) == 0 {
		return &Response{
			Success: false,
			Error:   "no legal moves available",
		}
	}

	// Select move based on AI type
	var moveIdx int
	switch cmd.AIType {
	case "greedy":
		moveIdx = selectGreedyMoveIndex(currentState, currentGenome, moves)
	case "random":
		fallthrough
	default:
		moveIdx = rand.Intn(len(moves))
	}

	// Get move info
	moveInfos := convertMoves(moves, currentState, currentGenome)
	aiMove := &moveInfos[moveIdx]
	aiMove.Index = moveIdx

	return &Response{
		Success: true,
		AIMove:  aiMove,
	}
}

// handleValidateGenome runs 5 random games to check for crashes.
func handleValidateGenome(cmd *Command) *Response {
	// Decode genome from base64
	var genomeB64 string
	if err := json.Unmarshal(cmd.Genome, &genomeB64); err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("invalid genome field: %v", err),
		}
	}

	bytecode, err := base64.StdEncoding.DecodeString(genomeB64)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("invalid base64 genome: %v", err),
		}
	}

	// Parse genome
	genome, err := engine.ParseGenome(bytecode)
	if err != nil {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("failed to parse genome: %v", err),
		}
	}

	// Run 5 games with random AI
	seed := uint64(cmd.Seed)
	if seed == 0 {
		seed = 12345
	}

	stats := simulation.RunBatch(genome, 5, simulation.RandomAI, 0, seed)

	// Check for errors
	if stats.Errors > 0 {
		return &Response{
			Success: false,
			Error:   fmt.Sprintf("genome crashed in %d of 5 games", stats.Errors),
		}
	}

	return &Response{Success: true}
}

// setupDeck creates and shuffles a standard 52-card deck.
func setupDeck(state *engine.GameState, seed uint64) {
	for suit := uint8(0); suit < 4; suit++ {
		for rank := uint8(0); rank < 13; rank++ {
			state.Deck = append(state.Deck, engine.Card{Rank: rank, Suit: suit})
		}
	}
	state.ShuffleDeck(seed)
}

// convertMoves converts engine.LegalMove to MoveInfo for JSON.
func convertMoves(moves []engine.LegalMove, state *engine.GameState, genome *engine.Genome) []MoveInfo {
	infos := make([]MoveInfo, len(moves))
	for i, move := range moves {
		infos[i] = MoveInfo{
			Index:     i,
			Label:     describeMoveLabel(move, state, genome),
			Type:      describeMoveType(move, genome),
			CardIndex: move.CardIndex,
		}
	}
	return infos
}

// describeMoveLabel creates a human-readable label for a move.
func describeMoveLabel(move engine.LegalMove, state *engine.GameState, genome *engine.Genome) string {
	if move.PhaseIndex >= len(genome.TurnPhases) {
		return "Unknown"
	}

	phase := genome.TurnPhases[move.PhaseIndex]
	currentPlayer := state.CurrentPlayer

	switch phase.PhaseType {
	case engine.PhaseTypeDraw:
		if move.CardIndex == engine.MoveDraw {
			return "Draw"
		} else if move.CardIndex == engine.MoveDrawPass {
			return "Stand"
		}
		return "Draw"

	case engine.PhaseTypePlay:
		if move.CardIndex == engine.MovePlayPass {
			return "Pass"
		}
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[currentPlayer].Hand) {
			card := state.Players[currentPlayer].Hand[move.CardIndex]
			return fmt.Sprintf("Play %s", cardName(card))
		}
		if move.CardIndex <= -100 {
			rank := uint8(-(move.CardIndex + 100))
			return fmt.Sprintf("Play set of %s", rankName(rank))
		}
		return "Play"

	case engine.PhaseTypeDiscard:
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[currentPlayer].Hand) {
			card := state.Players[currentPlayer].Hand[move.CardIndex]
			return fmt.Sprintf("Discard %s", cardName(card))
		}
		return "Discard"

	case engine.PhaseTypeTrick:
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[currentPlayer].Hand) {
			card := state.Players[currentPlayer].Hand[move.CardIndex]
			return fmt.Sprintf("Play %s", cardName(card))
		}
		return "Play to trick"

	case engine.PhaseTypeBetting:
		switch move.CardIndex {
		case engine.MoveBettingCheck:
			return "Check"
		case engine.MoveBettingBet:
			return "Bet"
		case engine.MoveBettingCall:
			return "Call"
		case engine.MoveBettingRaise:
			return "Raise"
		case engine.MoveBettingAllIn:
			return "All In"
		case engine.MoveBettingFold:
			return "Fold"
		}
		return "Bet"

	case engine.PhaseTypeClaim:
		if move.CardIndex == engine.MoveChallenge {
			return "Challenge"
		} else if move.CardIndex == engine.MovePass {
			return "Accept"
		}
		if move.CardIndex >= 0 && move.CardIndex < len(state.Players[currentPlayer].Hand) {
			claimedRank := uint8(state.TurnNumber % 13)
			return fmt.Sprintf("Claim %s", rankName(claimedRank))
		}
		return "Claim"

	case engine.PhaseTypeBidding:
		if move.CardIndex <= engine.MoveBidOffset {
			bidValue := engine.MoveBidOffset - move.CardIndex
			if move.TargetLoc == engine.LocationDiscard { // Nil marker
				return "Bid Nil"
			}
			return fmt.Sprintf("Bid %d", bidValue)
		}
		return "Bid"
	}

	return "Unknown"
}

// describeMoveType returns the type of move (for UI categorization).
func describeMoveType(move engine.LegalMove, genome *engine.Genome) string {
	if move.PhaseIndex >= len(genome.TurnPhases) {
		return "unknown"
	}

	phase := genome.TurnPhases[move.PhaseIndex]
	switch phase.PhaseType {
	case engine.PhaseTypeDraw:
		return "draw"
	case engine.PhaseTypePlay:
		return "play"
	case engine.PhaseTypeDiscard:
		return "discard"
	case engine.PhaseTypeTrick:
		return "trick"
	case engine.PhaseTypeBetting:
		return "betting"
	case engine.PhaseTypeClaim:
		return "claim"
	case engine.PhaseTypeBidding:
		return "bidding"
	}
	return "unknown"
}

// cardName returns a human-readable card name.
func cardName(card engine.Card) string {
	return fmt.Sprintf("%s%s", rankName(card.Rank), suitName(card.Suit))
}

// rankName returns the rank as a string.
func rankName(rank uint8) string {
	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	if int(rank) < len(ranks) {
		return ranks[rank]
	}
	return "?"
}

// suitName returns the suit as a symbol.
func suitName(suit uint8) string {
	suits := []string{"♥", "♦", "♣", "♠"}
	if int(suit) < len(suits) {
		return suits[suit]
	}
	return "?"
}

// serializeState converts GameState to SerializedState for JSON.
func serializeState(state *engine.GameState) *SerializedState {
	s := &SerializedState{
		CurrentPlayer:     int(state.CurrentPlayer),
		TurnNumber:        int(state.TurnNumber),
		WinnerID:          int(state.WinnerID),
		NumPlayers:        int(state.NumPlayers),
		Pot:               state.Pot,
		CurrentBet:        state.CurrentBet,
		BettingComplete:   state.BettingComplete,
		TrickLeader:       int(state.TrickLeader),
		HeartsBroken:      state.HeartsBroken,
		TableauMode:       int(state.TableauMode),
		SequenceDirection: int(state.SequenceDirection),
	}

	// Players
	numPlayers := int(state.NumPlayers)
	if numPlayers == 0 {
		numPlayers = 2
	}
	s.Players = make([]SerializedPlayer, numPlayers)
	for i := 0; i < numPlayers; i++ {
		p := &state.Players[i]
		sp := SerializedPlayer{
			Hand:       make([]SerializedCard, len(p.Hand)),
			Score:      int(p.Score),
			Active:     p.Active,
			Chips:      p.Chips,
			CurrentBet: p.CurrentBet,
			HasFolded:  p.HasFolded,
			IsAllIn:    p.IsAllIn,
		}
		for j, card := range p.Hand {
			sp.Hand[j] = SerializedCard{Rank: int(card.Rank), Suit: int(card.Suit)}
		}
		s.Players[i] = sp
	}

	// Deck
	s.Deck = make([]SerializedCard, len(state.Deck))
	for i, card := range state.Deck {
		s.Deck[i] = SerializedCard{Rank: int(card.Rank), Suit: int(card.Suit)}
	}

	// Discard
	s.Discard = make([]SerializedCard, len(state.Discard))
	for i, card := range state.Discard {
		s.Discard[i] = SerializedCard{Rank: int(card.Rank), Suit: int(card.Suit)}
	}

	// Tableau
	s.Tableau = make([][]SerializedCard, len(state.Tableau))
	for i, pile := range state.Tableau {
		s.Tableau[i] = make([]SerializedCard, len(pile))
		for j, card := range pile {
			s.Tableau[i][j] = SerializedCard{Rank: int(card.Rank), Suit: int(card.Suit)}
		}
	}

	// Current trick
	if len(state.CurrentTrick) > 0 {
		s.CurrentTrick = make([]SerializedTrickCard, len(state.CurrentTrick))
		for i, tc := range state.CurrentTrick {
			s.CurrentTrick[i] = SerializedTrickCard{
				PlayerID: int(tc.PlayerID),
				Card:     SerializedCard{Rank: int(tc.Card.Rank), Suit: int(tc.Card.Suit)},
			}
		}
	}

	// Tricks won
	if len(state.TricksWon) > 0 {
		s.TricksWon = make([]int, len(state.TricksWon))
		for i, tw := range state.TricksWon {
			s.TricksWon[i] = int(tw)
		}
	}

	return s
}

// deserializeState loads SerializedState back into GameState.
func deserializeState(s *SerializedState, state *engine.GameState) {
	state.Reset()

	state.CurrentPlayer = uint8(s.CurrentPlayer)
	state.TurnNumber = uint32(s.TurnNumber)
	state.WinnerID = int8(s.WinnerID)
	state.NumPlayers = uint8(s.NumPlayers)
	state.Pot = s.Pot
	state.CurrentBet = s.CurrentBet
	state.BettingComplete = s.BettingComplete
	state.TrickLeader = uint8(s.TrickLeader)
	state.HeartsBroken = s.HeartsBroken
	state.TableauMode = uint8(s.TableauMode)
	state.SequenceDirection = uint8(s.SequenceDirection)

	// Players
	for i, sp := range s.Players {
		if i >= len(state.Players) {
			break
		}
		p := &state.Players[i]
		p.Hand = make([]engine.Card, len(sp.Hand))
		for j, sc := range sp.Hand {
			p.Hand[j] = engine.Card{Rank: uint8(sc.Rank), Suit: uint8(sc.Suit)}
		}
		p.Score = int32(sp.Score)
		p.Active = sp.Active
		p.Chips = sp.Chips
		p.CurrentBet = sp.CurrentBet
		p.HasFolded = sp.HasFolded
		p.IsAllIn = sp.IsAllIn
	}

	// Deck
	state.Deck = make([]engine.Card, len(s.Deck))
	for i, sc := range s.Deck {
		state.Deck[i] = engine.Card{Rank: uint8(sc.Rank), Suit: uint8(sc.Suit)}
	}

	// Discard
	state.Discard = make([]engine.Card, len(s.Discard))
	for i, sc := range s.Discard {
		state.Discard[i] = engine.Card{Rank: uint8(sc.Rank), Suit: uint8(sc.Suit)}
	}

	// Tableau
	state.Tableau = make([][]engine.Card, len(s.Tableau))
	for i, pile := range s.Tableau {
		state.Tableau[i] = make([]engine.Card, len(pile))
		for j, sc := range pile {
			state.Tableau[i][j] = engine.Card{Rank: uint8(sc.Rank), Suit: uint8(sc.Suit)}
		}
	}

	// Current trick
	state.CurrentTrick = make([]engine.TrickCard, len(s.CurrentTrick))
	for i, tc := range s.CurrentTrick {
		state.CurrentTrick[i] = engine.TrickCard{
			PlayerID: uint8(tc.PlayerID),
			Card:     engine.Card{Rank: uint8(tc.Card.Rank), Suit: uint8(tc.Card.Suit)},
		}
	}

	// Tricks won
	state.TricksWon = make([]uint8, len(s.TricksWon))
	for i, tw := range s.TricksWon {
		state.TricksWon[i] = uint8(tw)
	}
}

// selectGreedyMoveIndex picks the best move using greedy heuristics.
func selectGreedyMoveIndex(state *engine.GameState, genome *engine.Genome, moves []engine.LegalMove) int {
	bestIdx := 0
	bestScore := scoreMove(state, &moves[0])

	for i := 1; i < len(moves); i++ {
		score := scoreMove(state, &moves[i])
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx
}

// scoreMove assigns a heuristic value to a move.
func scoreMove(state *engine.GameState, move *engine.LegalMove) float64 {
	score := 0.0

	// Prefer moves that reduce hand size
	if move.CardIndex >= 0 {
		score += 10.0
	}

	// Prefer playing higher ranked cards
	if move.CardIndex >= 0 && move.CardIndex < len(state.Players[state.CurrentPlayer].Hand) {
		card := state.Players[state.CurrentPlayer].Hand[move.CardIndex]
		score += float64(card.Rank)
	}

	return score
}

// writeResponse writes a JSON response to stdout.
func writeResponse(resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		writeError(fmt.Sprintf("failed to marshal response: %v", err))
		return
	}
	fmt.Println(string(data))
}

// writeError writes an error response to stdout.
func writeError(msg string) {
	resp := &Response{
		Success: false,
		Error:   msg,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
