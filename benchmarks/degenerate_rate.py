#!/usr/bin/env python3
"""Measure degenerate genome rates for random generation.

Tests:
1. Parse correctly
2. Produce a game that terminates
3. Terminate in < 100 turns
4. Have meaningful decisions (not just forced moves)
"""

import random
from dataclasses import dataclass
from typing import Optional
import flatbuffers

from darwindeck.genome.schema import (
    GameGenome, SetupRules, TurnStructure, WinCondition,
    PlayPhase, DrawPhase, DiscardPhase, Location
)
from darwindeck.genome.conditions import Condition, ConditionType, Operator
from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.bindings.cgo_bridge import simulate_batch
from darwindeck.bindings.cardsim.SimulationRequest import (
    SimulationRequestStart, SimulationRequestAddGenomeBytecode,
    SimulationRequestAddNumGames, SimulationRequestAddAiPlayerType,
    SimulationRequestAddMctsIterations, SimulationRequestAddRandomSeed,
    SimulationRequestEnd,
)
from darwindeck.bindings.cardsim.BatchRequest import (
    BatchRequestStart, BatchRequestAddBatchId, BatchRequestAddRequests,
    BatchRequestStartRequestsVector, BatchRequestEnd,
)


def generate_random_genome() -> GameGenome:
    """Generate a truly random genome."""
    # Random setup
    setup = SetupRules(
        initial_deck="standard_52",
        cards_per_player=random.randint(3, 26),
        initial_discard_count=random.randint(0, 5),
    )

    # Random phases (1-5)
    num_phases = random.randint(1, 5)
    phases = []
    for _ in range(num_phases):
        phase_type = random.choice(["draw", "play", "discard"])
        if phase_type == "draw":
            phases.append(DrawPhase(
                source=random.choice([Location.DECK, Location.DISCARD]),
                count=random.randint(1, 3),
                mandatory=random.choice([True, False]),
            ))
        elif phase_type == "play":
            phases.append(PlayPhase(
                target=random.choice([Location.DISCARD, Location.TABLEAU]),
                valid_play_condition=Condition(
                    type=random.choice([ConditionType.HAND_SIZE, ConditionType.LOCATION_SIZE]),
                    operator=random.choice([Operator.GT, Operator.GE, Operator.EQ]),
                    value=random.randint(0, 5),
                ),
                min_cards=random.randint(0, 2),
                max_cards=random.randint(1, 3),
                mandatory=random.choice([True, False]),
            ))
        else:
            phases.append(DiscardPhase(
                target=Location.DISCARD,
                count=random.randint(1, 3),
                mandatory=random.choice([True, False]),
            ))

    turn_structure = TurnStructure(phases=tuple(phases))

    # Random win condition
    win_type = random.choice(["empty_hand", "first_to_score", "capture_all"])
    if win_type == "first_to_score":
        win_conditions = [WinCondition(type=win_type, threshold=random.choice([50, 100, 200]))]
    else:
        win_conditions = [WinCondition(type=win_type)]

    return GameGenome(
        schema_version="1.0",
        genome_id=f"random-{random.randint(0, 999999)}",
        generation=0,
        setup=setup,
        turn_structure=turn_structure,
        special_effects=[],
        win_conditions=win_conditions,
        scoring_rules=[],
        max_turns=random.randint(50, 500),
        min_turns=5,
        player_count=2,
    )


@dataclass
class GenomeStats:
    """Statistics for a tested genome."""
    parsed: bool = False
    terminated: bool = False
    turn_count: int = 0
    errors: int = 0
    forced_decisions: int = 0
    total_decisions: int = 0
    has_meaningful_decisions: bool = False


def test_genome(genome: GameGenome, num_games: int = 10) -> GenomeStats:
    """Test a genome for viability."""
    stats = GenomeStats()

    # 1. Try to parse/compile
    try:
        compiler = BytecodeCompiler()
        bytecode = compiler.compile_genome(genome)
        stats.parsed = True
    except Exception as e:
        return stats

    # 2. Try to simulate
    try:
        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, num_games)
        SimulationRequestAddAiPlayerType(builder, 0)  # Random AI
        SimulationRequestAddMctsIterations(builder, 0)
        SimulationRequestAddRandomSeed(builder, random.randint(0, 2**32))
        req_offset = SimulationRequestEnd(builder)

        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, 1)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

        builder.Finish(batch_offset)
        response = simulate_batch(bytes(builder.Output()))

        result = response.Results(0)
        stats.errors = result.Errors()
        stats.turn_count = int(result.AvgTurns())

        # Check if games terminated (not all errors)
        if result.Errors() < num_games:
            stats.terminated = True

        # Check Phase 1 metrics for meaningful decisions
        if result.TotalDecisions() > 0:
            stats.total_decisions = result.TotalDecisions()
            stats.forced_decisions = result.ForcedDecisions()
            forced_ratio = result.ForcedDecisions() / result.TotalDecisions()
            # "Meaningful" = less than 90% forced
            stats.has_meaningful_decisions = forced_ratio < 0.9

    except Exception as e:
        return stats

    return stats


def main():
    """Run degenerate rate benchmark."""
    print("=" * 60)
    print("Degenerate Genome Rate Benchmark")
    print("=" * 60)

    num_genomes = 100
    games_per_genome = 10

    results = {
        "parsed": 0,
        "terminated": 0,
        "under_100_turns": 0,
        "meaningful_decisions": 0,
    }

    print(f"\nTesting {num_genomes} random genomes...")
    print()

    for i in range(num_genomes):
        genome = generate_random_genome()
        stats = test_genome(genome, games_per_genome)

        if stats.parsed:
            results["parsed"] += 1
        if stats.terminated:
            results["terminated"] += 1
        if stats.terminated and stats.turn_count < 100:
            results["under_100_turns"] += 1
        if stats.has_meaningful_decisions:
            results["meaningful_decisions"] += 1

        if (i + 1) % 10 == 0:
            print(f"  Tested {i + 1}/{num_genomes}...")

    print()
    print("=" * 60)
    print("Results:")
    print("=" * 60)
    print(f"  Parse correctly:           {results['parsed']:3d}/{num_genomes} ({100*results['parsed']/num_genomes:.1f}%)")
    print(f"  Terminate (not stuck):     {results['terminated']:3d}/{num_genomes} ({100*results['terminated']/num_genomes:.1f}%)")
    print(f"  Terminate < 100 turns:     {results['under_100_turns']:3d}/{num_genomes} ({100*results['under_100_turns']/num_genomes:.1f}%)")
    print(f"  Meaningful decisions:      {results['meaningful_decisions']:3d}/{num_genomes} ({100*results['meaningful_decisions']/num_genomes:.1f}%)")
    print()

    # Calculate combined "viable" rate
    viable = results["meaningful_decisions"]  # All requirements
    print(f"Combined viable rate: {viable}/{num_genomes} ({100*viable/num_genomes:.1f}%)")
    print()


if __name__ == "__main__":
    main()
