"""MCTS skill evaluation for evolved games.

Measures skill gap by running MCTS vs Random head-to-head games
in both directions (MCTS as P1, then as P2) to eliminate first-player bias.
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
    """Result of MCTS skill evaluation for a single genome."""
    genome_id: str
    mcts_wins_as_p1: int      # MCTS wins when playing as Player 1
    mcts_wins_as_p2: int      # MCTS wins when playing as Player 2
    total_mcts_wins: int      # Combined wins
    total_games: int          # Total games played
    mcts_win_rate: float      # total_mcts_wins / total_games (0.0-1.0)
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
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    progress_callback: Optional[Callable[[str], None]] = None
) -> SkillEvalResult:
    """Run symmetric MCTS vs Random evaluation.

    Runs num_games/2 with MCTS as P1, num_games/2 with MCTS as P2.
    This eliminates first-player advantage bias.

    Args:
        genome: Game genome to evaluate
        num_games: Total games to play (split evenly between directions)
        mcts_iterations: MCTS search iterations per move
        timeout_sec: Maximum time for entire evaluation
        progress_callback: Optional callback for progress updates

    Returns:
        SkillEvalResult with win rates and timing info
    """
    start_time = time.time()
    simulator = GoSimulator()

    games_per_direction = num_games // 2

    # Determine MCTS AI type based on iterations
    if mcts_iterations >= 2000:
        mcts_type = "mcts2000"
    elif mcts_iterations >= 1000:
        mcts_type = "mcts1000"
    elif mcts_iterations >= 500:
        mcts_type = "mcts500"
    else:
        mcts_type = "mcts"  # mcts100

    # Direction 1: MCTS as P0, Random as P1
    if progress_callback:
        progress_callback(f"Running MCTS as P0...")

    result_p0 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type=mcts_type,
        p1_ai_type="random",
        mcts_iterations=mcts_iterations
    )

    # Check timeout
    elapsed = time.time() - start_time
    if elapsed > timeout_sec:
        mcts_wins = result_p0.player0_wins
        total = games_per_direction
        return SkillEvalResult(
            genome_id=genome.genome_id,
            mcts_wins_as_p1=mcts_wins,
            mcts_wins_as_p2=0,
            total_mcts_wins=mcts_wins,
            total_games=total,
            mcts_win_rate=mcts_wins / total if total > 0 else 0.5,
            timed_out=True
        )

    # Direction 2: Random as P0, MCTS as P1
    if progress_callback:
        progress_callback(f"Running MCTS as P1...")

    result_p1 = simulator.simulate_asymmetric(
        genome=genome,
        num_games=games_per_direction,
        p0_ai_type="random",
        p1_ai_type=mcts_type,
        mcts_iterations=mcts_iterations
    )

    # Combine results
    mcts_wins_as_p0 = result_p0.player0_wins  # MCTS was P0
    mcts_wins_as_p1 = result_p1.player1_wins  # MCTS was P1
    total_wins = mcts_wins_as_p0 + mcts_wins_as_p1
    total_games_played = games_per_direction * 2

    # Check for errors - if all games errored, use 0.5 as neutral
    total_errors = result_p0.errors + result_p1.errors
    if total_errors >= total_games_played:
        return SkillEvalResult(
            genome_id=genome.genome_id,
            mcts_wins_as_p1=0,
            mcts_wins_as_p2=0,
            total_mcts_wins=0,
            total_games=total_games_played,
            mcts_win_rate=0.5,  # Neutral when all errors
            timed_out=False
        )

    return SkillEvalResult(
        genome_id=genome.genome_id,
        mcts_wins_as_p1=mcts_wins_as_p0,  # When MCTS was "first" (P0)
        mcts_wins_as_p2=mcts_wins_as_p1,  # When MCTS was "second" (P1)
        total_mcts_wins=total_wins,
        total_games=total_games_played,
        mcts_win_rate=total_wins / total_games_played if total_games_played > 0 else 0.5,
        timed_out=False
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
    mcts_iterations: int = 500,
    timeout_sec: float = 60.0,
    num_workers: Optional[int] = None,
    progress_callback: Optional[Callable[[int, int], None]] = None
) -> List[SkillEvalResult]:
    """Evaluate skill gap for multiple genomes in parallel.

    Args:
        genomes: List of genomes to evaluate
        num_games: Games per genome (split between directions)
        mcts_iterations: MCTS search iterations
        timeout_sec: Timeout per genome
        num_workers: Worker processes (default: CPU count)
        progress_callback: Called with (completed, total) for progress

    Returns:
        List of SkillEvalResult, one per genome (same order)
    """
    if not genomes:
        return []

    num_workers = num_workers or int(os.environ.get('EVOLUTION_WORKERS', os.cpu_count() or 4))

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
