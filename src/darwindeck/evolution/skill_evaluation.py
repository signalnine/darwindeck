"""Two-tier skill evaluation for evolved games.

Measures skill gap using two AI comparisons:
1. Greedy vs Random - measures if basic strategy helps (fast)
2. MCTS vs Random - measures skill ceiling (moderate speed)

Both run in both directions to eliminate first-player bias.
"""

from dataclasses import dataclass
from typing import List, Optional, Callable, Dict
import logging
import time
import multiprocessing as mp
import os

from darwindeck.genome.schema import GameGenome
from darwindeck.simulation.go_simulator import GoSimulator

logger = logging.getLogger(__name__)

# Use 'spawn' context for CGo compatibility
_mp_context = mp.get_context('spawn')


@dataclass
class SkillEvalResult:
    """Result of two-tier skill evaluation for a single genome."""
    genome_id: str
    # Greedy vs Random results
    greedy_wins_as_p0: int    # Greedy wins when playing as Player 0
    greedy_wins_as_p1: int    # Greedy wins when playing as Player 1
    greedy_win_rate: float    # Combined greedy win rate (0.0-1.0)
    # MCTS vs Random results
    mcts_wins_as_p0: int      # MCTS wins when playing as Player 0
    mcts_wins_as_p1: int      # MCTS wins when playing as Player 1
    mcts_win_rate: float      # Combined MCTS win rate (0.0-1.0)
    # Combined
    total_games: int          # Total games played (greedy + mcts)
    skill_score: float        # Combined skill metric
    first_player_advantage: float  # 0.0 = balanced, 1.0 = P0 always wins, -1.0 = P1 always wins
    timed_out: bool = False   # True if evaluation was cut short


@dataclass
class _SkillEvalTask:
    """Task for parallel skill evaluation."""
    genome: GameGenome
    num_games: int
    mcts_iterations: int
    timeout_sec: float


def evaluate_skill(
    genome: GameGenome,
    num_games: int = 100,
    mcts_iterations: int = 100,
    timeout_sec: float = 60.0,
    progress_callback: Optional[Callable[[str], None]] = None
) -> SkillEvalResult:
    """Run two-tier skill evaluation: Greedy vs Random, then MCTS vs Random.

    Each tier runs num_games/2 in each direction to eliminate first-player bias.
    Total games = num_games * 2 (half for greedy, half for mcts).

    Args:
        genome: Game genome to evaluate
        num_games: Games per tier (split between directions), total = 2x this
        mcts_iterations: MCTS search iterations per move (default: 100)
        timeout_sec: Maximum time for entire evaluation
        progress_callback: Optional callback for progress updates

    Returns:
        SkillEvalResult with greedy and mcts win rates
    """
    start_time = time.time()
    simulator = GoSimulator()

    games_per_direction = num_games // 2

    # === Tier 1: Greedy vs Random (fast) ===
    if progress_callback:
        progress_callback("Running Greedy vs Random...")

    # Greedy as P0
    greedy_p0 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="greedy",
        p1_ai_type="random"
    )

    # Check timeout
    if time.time() - start_time > timeout_sec:
        return _make_timeout_result(genome.genome_id, greedy_p0, None, None, None, games_per_direction)

    # Greedy as P1
    greedy_p1 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="random",
        p1_ai_type="greedy"
    )

    if time.time() - start_time > timeout_sec:
        return _make_timeout_result(genome.genome_id, greedy_p0, greedy_p1, None, None, games_per_direction)

    # === Tier 2: MCTS vs Random ===
    if progress_callback:
        progress_callback("Running MCTS vs Random...")

    # Determine MCTS AI type based on iterations
    if mcts_iterations >= 2000:
        mcts_type = "mcts2000"
    elif mcts_iterations >= 1000:
        mcts_type = "mcts1000"
    elif mcts_iterations >= 500:
        mcts_type = "mcts500"
    else:
        mcts_type = "mcts"  # mcts100

    # MCTS as P0
    mcts_p0 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type=mcts_type,
        p1_ai_type="random",
        mcts_iterations=mcts_iterations
    )

    if time.time() - start_time > timeout_sec:
        return _make_timeout_result(genome.genome_id, greedy_p0, greedy_p1, mcts_p0, None, games_per_direction)

    # MCTS as P1
    mcts_p1 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="random",
        p1_ai_type=mcts_type,
        mcts_iterations=mcts_iterations
    )

    # Combine results
    greedy_wins_p0 = greedy_p0.player0_wins
    greedy_wins_p1 = greedy_p1.player1_wins
    mcts_wins_p0 = mcts_p0.player0_wins
    mcts_wins_p1 = mcts_p1.player1_wins

    total_greedy_games = games_per_direction * 2
    total_mcts_games = games_per_direction * 2
    total_games = total_greedy_games + total_mcts_games

    # Check for errors
    greedy_errors = greedy_p0.errors + greedy_p1.errors
    mcts_errors = mcts_p0.errors + mcts_p1.errors

    if greedy_errors >= total_greedy_games and mcts_errors >= total_mcts_games:
        return SkillEvalResult(
            genome_id=genome.genome_id,
            greedy_wins_as_p0=0, greedy_wins_as_p1=0, greedy_win_rate=0.5,
            mcts_wins_as_p0=0, mcts_wins_as_p1=0, mcts_win_rate=0.5,
            total_games=total_games,
            skill_score=0.5,
            first_player_advantage=0.0,
            timed_out=False
        )

    # Calculate win rates
    greedy_win_rate = (greedy_wins_p0 + greedy_wins_p1) / total_greedy_games if total_greedy_games > 0 else 0.5
    mcts_win_rate = (mcts_wins_p0 + mcts_wins_p1) / total_mcts_games if total_mcts_games > 0 else 0.5

    # Combined skill score: weighted average
    # Greedy measures basic skill, MCTS measures skill ceiling
    skill_score = greedy_win_rate * 0.5 + mcts_win_rate * 0.5

    # Calculate first player advantage
    # Compare P0 win rate vs P1 win rate across all tests
    # Positive = P0 advantage, Negative = P1 advantage, 0 = balanced
    p0_win_rate = (greedy_wins_p0 + mcts_wins_p0) / (games_per_direction * 2) if games_per_direction > 0 else 0.5
    p1_win_rate = (greedy_wins_p1 + mcts_wins_p1) / (games_per_direction * 2) if games_per_direction > 0 else 0.5
    # Scale to -1..1 range: (p0 - p1) where both are 0..1
    first_player_advantage = p0_win_rate - p1_win_rate

    return SkillEvalResult(
        genome_id=genome.genome_id,
        greedy_wins_as_p0=greedy_wins_p0,
        greedy_wins_as_p1=greedy_wins_p1,
        greedy_win_rate=greedy_win_rate,
        mcts_wins_as_p0=mcts_wins_p0,
        mcts_wins_as_p1=mcts_wins_p1,
        mcts_win_rate=mcts_win_rate,
        total_games=total_games,
        skill_score=skill_score,
        first_player_advantage=first_player_advantage,
        timed_out=False
    )


def _make_timeout_result(genome_id, greedy_p0, greedy_p1, mcts_p0, mcts_p1, games_per_direction) -> SkillEvalResult:
    """Create a partial result when timeout occurs."""
    greedy_wins_p0 = greedy_p0.player0_wins if greedy_p0 else 0
    greedy_wins_p1 = greedy_p1.player1_wins if greedy_p1 else 0
    mcts_wins_p0 = mcts_p0.player0_wins if mcts_p0 else 0
    mcts_wins_p1 = mcts_p1.player1_wins if mcts_p1 else 0

    greedy_games = (games_per_direction if greedy_p0 else 0) + (games_per_direction if greedy_p1 else 0)
    mcts_games = (games_per_direction if mcts_p0 else 0) + (games_per_direction if mcts_p1 else 0)

    greedy_win_rate = (greedy_wins_p0 + greedy_wins_p1) / greedy_games if greedy_games > 0 else 0.5
    mcts_win_rate = (mcts_wins_p0 + mcts_wins_p1) / mcts_games if mcts_games > 0 else 0.5
    skill_score = greedy_win_rate * 0.5 + mcts_win_rate * 0.5

    # Calculate first player advantage from available data
    p0_games = (games_per_direction if greedy_p0 else 0) + (games_per_direction if mcts_p0 else 0)
    p1_games = (games_per_direction if greedy_p1 else 0) + (games_per_direction if mcts_p1 else 0)
    p0_win_rate = (greedy_wins_p0 + mcts_wins_p0) / p0_games if p0_games > 0 else 0.5
    p1_win_rate = (greedy_wins_p1 + mcts_wins_p1) / p1_games if p1_games > 0 else 0.5
    first_player_advantage = p0_win_rate - p1_win_rate

    return SkillEvalResult(
        genome_id=genome_id,
        greedy_wins_as_p0=greedy_wins_p0,
        greedy_wins_as_p1=greedy_wins_p1,
        greedy_win_rate=greedy_win_rate,
        mcts_wins_as_p0=mcts_wins_p0,
        mcts_wins_as_p1=mcts_wins_p1,
        mcts_win_rate=mcts_win_rate,
        total_games=greedy_games + mcts_games,
        skill_score=skill_score,
        first_player_advantage=first_player_advantage,
        timed_out=True
    )


def _evaluate_skill_task(task: _SkillEvalTask) -> SkillEvalResult:
    """Worker function for parallel evaluation."""
    return evaluate_skill(
        genome=task.genome,
        num_games=task.num_games,
        mcts_iterations=task.mcts_iterations,
        timeout_sec=task.timeout_sec
    )


def evaluate_batch_skill(
    genomes: List[GameGenome],
    num_games: int = 100,
    mcts_iterations: int = 100,
    timeout_sec: float = 60.0,
    num_workers: Optional[int] = None,
    progress_callback: Optional[Callable[[int, int], None]] = None
) -> List[SkillEvalResult]:
    """Evaluate skill gap for multiple genomes in parallel.

    Uses two-tier evaluation: Greedy vs Random + MCTS vs Random.

    Args:
        genomes: List of genomes to evaluate
        num_games: Games per tier per genome (total = 2x this)
        mcts_iterations: MCTS search iterations (default: 100)
        timeout_sec: Timeout per genome
        num_workers: Worker processes (default: CPU count)
        progress_callback: Called with (completed, total) for progress

    Returns:
        List of SkillEvalResult, one per genome (same order)
    """
    if not genomes:
        return []

    # Cap default workers at 64 to avoid massive spawn overhead on high-core machines
    default_workers = min(os.cpu_count() or 4, 64)
    num_workers = num_workers or int(os.environ.get('EVOLUTION_WORKERS', default_workers))

    tasks = [
        _SkillEvalTask(genome, num_games, mcts_iterations, timeout_sec)
        for genome in genomes
    ]

    results: List[SkillEvalResult] = []

    with _mp_context.Pool(processes=num_workers) as pool:
        for i, result in enumerate(pool.imap(_evaluate_skill_task, tasks)):
            results.append(result)
            if progress_callback:
                progress_callback(i + 1, len(genomes))

    return results
