"""Go simulator wrapper using CGo bridge."""

import flatbuffers
from typing import Optional

from darwindeck.genome.schema import GameGenome
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
from darwindeck.evolution.fitness_full import SimulationResults


class GoSimulator:
    """Wrapper for Go simulation engine via CGo."""

    def __init__(self, seed: Optional[int] = None):
        """Initialize Go simulator.

        Args:
            seed: Random seed for reproducibility (optional)
        """
        self.compiler = BytecodeCompiler()
        self.seed = seed or 42
        self._batch_id = 0

    def simulate(
        self,
        genome: GameGenome,
        num_games: int = 100,
        use_mcts: bool = False,
        mcts_iterations: int = 100
    ) -> SimulationResults:
        """Simulate games with the given genome.

        Args:
            genome: Game genome to simulate
            num_games: Number of games to run
            use_mcts: Whether to use MCTS AI (slower but measures skill)
            mcts_iterations: MCTS iterations if use_mcts is True

        Returns:
            SimulationResults with game statistics and Phase 1 metrics
        """
        # Compile genome to bytecode
        try:
            bytecode = self.compiler.compile_genome(genome)
        except Exception as e:
            # Return error results for invalid genomes
            return SimulationResults(
                total_games=num_games,
                player0_wins=0,
                player1_wins=0,
                draws=0,
                avg_turns=0.0,
                errors=num_games,
            )

        # Build FlatBuffers request
        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, num_games)
        SimulationRequestAddAiPlayerType(builder, 2 if use_mcts else 0)  # MCTS100 or Random
        SimulationRequestAddMctsIterations(builder, mcts_iterations if use_mcts else 0)
        SimulationRequestAddRandomSeed(builder, self.seed + self._batch_id)
        req_offset = SimulationRequestEnd(builder)

        BatchRequestStartRequestsVector(builder, 1)
        builder.PrependUOffsetTRelative(req_offset)
        requests_offset = builder.EndVector()

        self._batch_id += 1
        BatchRequestStart(builder)
        BatchRequestAddBatchId(builder, self._batch_id)
        BatchRequestAddRequests(builder, requests_offset)
        batch_offset = BatchRequestEnd(builder)

        builder.Finish(batch_offset)

        # Call Go simulator
        try:
            response = simulate_batch(bytes(builder.Output()))
            result = response.Results(0)

            return SimulationResults(
                total_games=result.TotalGames(),
                player0_wins=result.Player0Wins(),
                player1_wins=result.Player1Wins(),
                draws=result.Draws(),
                avg_turns=result.AvgTurns(),
                errors=result.Errors(),
                # Phase 1 instrumentation
                total_decisions=result.TotalDecisions(),
                total_valid_moves=result.TotalValidMoves(),
                forced_decisions=result.ForcedDecisions(),
                total_interactions=result.TotalInteractions(),
                total_actions=result.TotalActions(),
            )
        except Exception as e:
            # Return error results for simulation failures
            return SimulationResults(
                total_games=num_games,
                player0_wins=0,
                player1_wins=0,
                draws=0,
                avg_turns=0.0,
                errors=num_games,
            )
