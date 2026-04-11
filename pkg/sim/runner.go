package sim

import "github.com/darwindeck/darwindeck/pkg/genome"

// GameResult holds the outcome of a single game.
type GameResult struct {
	Winner    int // Player ID who won, or -1 for draw
	Turns     int
	Events    []Event
	Error     string
	HandSizes []int // Final hand sizes per player
}

// SheddingRunnerInterface matches what the shedding runner provides.
// This avoids a circular import — the sim package defines the contract,
// the skeleton package implements it.
type SheddingRunnerInterface interface {
	Setup(g *genome.Genome, rng interface{}) *GameState
	GenerateMoves(state *GameState, g *genome.Genome) []Move
	ApplyMove(state *GameState, move Move, g *genome.Genome) []Event
	CheckEnd(state *GameState, g *genome.Genome) int
}
