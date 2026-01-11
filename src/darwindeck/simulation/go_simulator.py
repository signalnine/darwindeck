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
    SimulationRequestAddPlayer0AiType, SimulationRequestAddPlayer1AiType,
    SimulationRequestAddAiTypes, SimulationRequestStartAiTypesVector,
    SimulationRequestAddPlayerCount, SimulationRequestEnd,
)

# AI type mapping for asymmetric simulation
# Values are offset by 1 because 0 means "use default"
AI_TYPE_MAP = {
    "random": 1,    # RandomAI = 0, so offset = 1
    "greedy": 2,    # GreedyAI = 1, so offset = 2
    "mcts": 3,      # MCTS100AI = 2, so offset = 3
    "mcts100": 3,
    "mcts500": 4,   # MCTS500AI = 3, so offset = 4
    "mcts1000": 5,  # MCTS1000AI = 4, so offset = 5
    "mcts2000": 6,  # MCTS2000AI = 5, so offset = 6
}
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
        self._bytecode_cache: dict[str, bytes] = {}  # Cache compiled bytecode by genome_id

    def simulate(
        self,
        genome: GameGenome,
        num_games: int = 100,
        use_mcts: bool = False,
        mcts_iterations: int = 100,
        player_count: int = 2
    ) -> SimulationResults:
        """Simulate games with the given genome.

        Args:
            genome: Game genome to simulate
            num_games: Number of games to run
            use_mcts: Whether to use MCTS AI (slower but measures skill)
            mcts_iterations: MCTS iterations if use_mcts is True
            player_count: Number of players (2-4)

        Returns:
            SimulationResults with game statistics and Phase 1 metrics
        """
        # Validate player count
        if player_count < 2 or player_count > 4:
            player_count = 2

        # Compile genome to bytecode (with caching)
        try:
            cache_key = genome.genome_id
            if cache_key in self._bytecode_cache:
                bytecode = self._bytecode_cache[cache_key]
            else:
                bytecode = self.compiler.compile_genome(genome)
                self._bytecode_cache[cache_key] = bytecode
        except Exception as e:
            # Return error results for invalid genomes
            return SimulationResults(
                total_games=num_games,
                wins=tuple(0 for _ in range(player_count)),
                player_count=player_count,
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
        SimulationRequestAddPlayerCount(builder, player_count)
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

            # Read wins array, falling back to legacy fields if array is empty
            wins_len = result.WinsLength()
            if wins_len > 0:
                wins = tuple(result.Wins(i) for i in range(wins_len))
            else:
                # Fallback to legacy fields for backward compatibility
                wins = (result.Player0Wins(), result.Player1Wins())

            result_player_count = result.PlayerCount()
            if result_player_count == 0:
                result_player_count = 2

            return SimulationResults(
                total_games=result.TotalGames(),
                wins=wins,
                player_count=result_player_count,
                draws=result.Draws(),
                avg_turns=result.AvgTurns(),
                errors=result.Errors(),
                # Phase 1 instrumentation
                total_decisions=result.TotalDecisions(),
                total_valid_moves=result.TotalValidMoves(),
                forced_decisions=result.ForcedDecisions(),
                total_hand_size=result.TotalHandSize(),
                total_interactions=result.TotalInteractions(),
                total_actions=result.TotalActions(),
                # Bluffing metrics
                total_claims=result.TotalClaims(),
                total_bluffs=result.TotalBluffs(),
                total_challenges=result.TotalChallenges(),
                successful_bluffs=result.SuccessfulBluffs(),
                successful_catches=result.SuccessfulCatches(),
            )
        except Exception as e:
            # Return error results for simulation failures
            return SimulationResults(
                total_games=num_games,
                wins=tuple(0 for _ in range(player_count)),
                player_count=player_count,
                draws=0,
                avg_turns=0.0,
                errors=num_games,
            )

    def simulate_asymmetric(
        self,
        genome: GameGenome,
        num_games: int = 100,
        ai_types: Optional[list[str]] = None,
        mcts_iterations: int = 500,
        player_count: int = 2,
        # Legacy parameters for backward compatibility
        p0_ai_type: Optional[str] = None,
        p1_ai_type: Optional[str] = None,
    ) -> SimulationResults:
        """Simulate games with different AI types for each player.

        Used for skill gap measurement (e.g., MCTS vs Random).

        Args:
            genome: Game genome to simulate
            num_games: Number of games to run
            ai_types: AI type per player (list of "random", "greedy", "mcts", etc.)
            mcts_iterations: MCTS iterations (used if any player is MCTS)
            player_count: Number of players (2-4)
            p0_ai_type: (DEPRECATED) AI for player 0
            p1_ai_type: (DEPRECATED) AI for player 1

        Returns:
            SimulationResults with game statistics
        """
        # Validate player count
        if player_count < 2 or player_count > 4:
            player_count = 2

        # Handle ai_types parameter
        if ai_types is None:
            # Fallback to legacy parameters
            ai_types = [
                p0_ai_type or "random",
                p1_ai_type or "random",
            ]
            # Extend to player_count if needed
            while len(ai_types) < player_count:
                ai_types.append("random")

        # Compile genome to bytecode (with caching)
        try:
            cache_key = genome.genome_id
            if cache_key in self._bytecode_cache:
                bytecode = self._bytecode_cache[cache_key]
            else:
                bytecode = self.compiler.compile_genome(genome)
                self._bytecode_cache[cache_key] = bytecode
        except Exception as e:
            return SimulationResults(
                total_games=num_games,
                wins=tuple(0 for _ in range(player_count)),
                player_count=player_count,
                draws=0,
                avg_turns=0.0,
                errors=num_games,
            )

        # Map AI type strings to enum values (with offset)
        ai_type_values = [
            AI_TYPE_MAP.get(ai.lower(), 1) for ai in ai_types[:player_count]
        ]

        builder = flatbuffers.Builder(2048)
        genome_offset = builder.CreateByteVector(bytecode)

        # Build ai_types vector
        SimulationRequestStartAiTypesVector(builder, len(ai_type_values))
        for ai_val in reversed(ai_type_values):
            builder.PrependUint8(ai_val)
        ai_types_offset = builder.EndVector()

        SimulationRequestStart(builder)
        SimulationRequestAddGenomeBytecode(builder, genome_offset)
        SimulationRequestAddNumGames(builder, num_games)
        SimulationRequestAddAiPlayerType(builder, 0)  # Not used when per-player set
        SimulationRequestAddMctsIterations(builder, mcts_iterations)
        SimulationRequestAddRandomSeed(builder, self.seed + self._batch_id)
        SimulationRequestAddAiTypes(builder, ai_types_offset)
        SimulationRequestAddPlayerCount(builder, player_count)
        # Also set legacy fields for backward compatibility with older Go code
        SimulationRequestAddPlayer0AiType(builder, ai_type_values[0] if ai_type_values else 1)
        SimulationRequestAddPlayer1AiType(builder, ai_type_values[1] if len(ai_type_values) > 1 else 1)
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

        try:
            response = simulate_batch(bytes(builder.Output()))
            result = response.Results(0)

            # Read wins array, falling back to legacy fields if array is empty
            wins_len = result.WinsLength()
            if wins_len > 0:
                wins = tuple(result.Wins(i) for i in range(wins_len))
            else:
                # Fallback to legacy fields for backward compatibility
                wins = (result.Player0Wins(), result.Player1Wins())

            result_player_count = result.PlayerCount()
            if result_player_count == 0:
                result_player_count = 2

            return SimulationResults(
                total_games=result.TotalGames(),
                wins=wins,
                player_count=result_player_count,
                draws=result.Draws(),
                avg_turns=result.AvgTurns(),
                errors=result.Errors(),
                total_decisions=result.TotalDecisions(),
                total_valid_moves=result.TotalValidMoves(),
                forced_decisions=result.ForcedDecisions(),
                total_hand_size=result.TotalHandSize(),
                total_interactions=result.TotalInteractions(),
                total_actions=result.TotalActions(),
                # Bluffing metrics
                total_claims=result.TotalClaims(),
                total_bluffs=result.TotalBluffs(),
                total_challenges=result.TotalChallenges(),
                successful_bluffs=result.SuccessfulBluffs(),
                successful_catches=result.SuccessfulCatches(),
            )
        except Exception as e:
            return SimulationResults(
                total_games=num_games,
                wins=tuple(0 for _ in range(player_count)),
                player_count=player_count,
                draws=0,
                avg_turns=0.0,
                errors=num_games,
            )
