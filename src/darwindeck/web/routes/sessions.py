"""Sessions API routes for game play."""

from __future__ import annotations

import base64
import json
import uuid
from datetime import datetime, timezone
from typing import Any, Optional

from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel, ConfigDict
from sqlalchemy.orm import Session as SQLSession

from darwindeck.genome.bytecode import BytecodeCompiler
from darwindeck.genome.serialization import genome_from_dict
from darwindeck.web.models import Game, GameSession
from darwindeck.web.dependencies import get_db, get_worker
from darwindeck.web.worker import SimulationWorker, SimulationError
from darwindeck.web.security import get_real_ip, hash_ip

# Bytecode compiler for genome conversion
_bytecode_compiler = BytecodeCompiler()


def _check_win_from_state(raw_state: dict[str, Any]) -> int:
    """Check if game is won based on raw state.

    Returns winner player index (0+) or -1 if no winner.
    Currently checks for empty_hand win condition (first player with 0 cards wins).
    """
    players = raw_state.get("players", [])
    if not players:
        return -1

    # Check for empty_hand win condition: first player with 0 cards wins
    for i, player in enumerate(players):
        hand = player.get("hand", [])
        if len(hand) == 0:
            return i

    return -1


def _go_card_to_frontend_int(go_rank: int, go_suit: int) -> int:
    """Convert Go card encoding to frontend card integer.

    Go uses:
      rank 0-12 where 0=Two, 1=Three, ..., 11=King, 12=Ace
      suit 0-3 where 0=Hearts, 1=Diamonds, 2=Clubs, 3=Spades

    Frontend uses:
      rank 1-13 where 1=Ace, 2=Two, ..., 12=Queen, 13=King
      suit 0-3 where 0=Clubs, 1=Diamonds, 2=Hearts, 3=Spades
      cardInt = (rank - 1) * 4 + suit
    """
    # Convert rank: Go Ace (12) → Frontend Ace (1), others add 2
    if go_rank == 12:  # Ace in Go
        frontend_rank = 1  # Ace in frontend
    else:
        frontend_rank = go_rank + 2  # Two (0→2), Three (1→3), ..., King (11→13)

    # Convert suit: Go {0:H, 1:D, 2:C, 3:S} → Frontend {0:C, 1:D, 2:H, 3:S}
    suit_map = {0: 2, 1: 1, 2: 0, 3: 3}  # Hearts↔Hearts, Diamonds↔Diamonds, Clubs↔Clubs, Spades↔Spades
    frontend_suit = suit_map.get(go_suit, go_suit)

    return (frontend_rank - 1) * 4 + frontend_suit


def _transform_worker_state(worker_result: dict[str, Any]) -> dict[str, Any]:
    """Transform worker's response to match frontend GameState schema.

    Worker returns:
        - state: SerializedState with players, deck, tableau, etc.
        - moves: array of MoveInfo with index, label, type
        - winner: int

    Frontend expects:
        - hands: number[][] (cards as rank*4 + suit integers)
        - deck_size: number
        - phase: string
        - active_player: number
        - turn: number
        - scores: number[]
        - tableau: number[][] (cards as integers)
        - winner: number | null
        - legal_moves: Move[] (with index, type, card_index)
        - chips, pot, current_bet for betting games
    """
    state = worker_result.get("state", {})
    moves = worker_result.get("moves", [])
    winner = worker_result.get("winner", -1)

    # Convert players' hands to card integers
    players = state.get("players", [])
    hands: list[list[int]] = []
    scores: list[int] = []
    chips: list[int] = []

    for player in players:
        hand = player.get("hand", [])
        # Convert Go {rank, suit} to frontend card integer
        hand_ints = [_go_card_to_frontend_int(card["rank"], card["suit"]) for card in hand]
        hands.append(hand_ints)
        scores.append(player.get("score", 0))
        chips.append(int(player.get("chips", 0)))

    # Convert tableau to card integers
    tableau_raw = state.get("tableau", [])
    tableau: list[list[int]] = []
    for pile in tableau_raw:
        pile_ints = [_go_card_to_frontend_int(card["rank"], card["suit"]) for card in pile]
        tableau.append(pile_ints)

    # Convert moves to frontend format
    legal_moves: list[dict[str, Any]] = []
    for move in moves:
        legal_moves.append({
            "index": move.get("index", 0),
            "type": move.get("type", "unknown"),
            "label": move.get("label", ""),
            "card_index": move.get("card_index", -1),
        })

    return {
        "hands": hands,
        "deck_size": len(state.get("deck", [])),
        "phase": "play",  # TODO: get from genome/state
        "phase_index": 0,
        "active_player": state.get("current_player", 0),
        "turn": state.get("turn_number", 0),
        "scores": scores,
        "tableau": tableau,
        "captured": [[] for _ in players],  # Not tracked in worker state
        "winner": winner if winner >= 0 else None,
        "legal_moves": legal_moves,
        "chips": chips if any(c > 0 for c in chips) else None,
        "pot": state.get("pot", 0) or None,
        "current_bet": state.get("current_bet", 0) or None,
        "version": 1,  # Will be overwritten
    }


router = APIRouter()


# Request/Response models


class StartGameResponse(BaseModel):
    """Response from starting a new game session."""

    session_id: str
    state: dict[str, Any]
    version: int
    genome_name: str


class SessionResponse(BaseModel):
    """Response for session retrieval."""

    model_config = ConfigDict(from_attributes=True)

    session_id: str
    game_id: str
    state: dict[str, Any]
    version: int
    completed: bool


class MoveRequest(BaseModel):
    """Request to apply a move."""

    move: dict[str, Any]
    version: int  # Optimistic locking - must match current version


class MoveResponse(BaseModel):
    """Response after applying a move."""

    state: dict[str, Any]
    version: int
    completed: bool
    ai_moves: list[dict[str, Any]] = []


class FlagRequest(BaseModel):
    """Request to flag a broken session."""

    reason: Optional[str] = None


class FlagResponse(BaseModel):
    """Response after flagging a session."""

    flagged: bool


# Endpoints


@router.post("/games/{game_id}/start", response_model=StartGameResponse)
async def start_game(
    game_id: str,
    request: Request,
    db: SQLSession = Depends(get_db),
    worker: SimulationWorker = Depends(get_worker),
):
    """Start a new game session.

    Creates a new GameSession, calls the simulation worker to initialize
    the game state, and returns the session ID with initial state.
    """
    # Verify game exists
    game = db.query(Game).filter(Game.id == game_id).first()
    if not game:
        raise HTTPException(status_code=404, detail="Game not found")

    # Get player IP for tracking
    ip = get_real_ip(request)
    ip_hash = hash_ip(ip)

    # Compile genome to bytecode for Go worker
    try:
        genome_dict = json.loads(game.genome_json)
        genome = genome_from_dict(genome_dict)
        bytecode = _bytecode_compiler.compile_genome(genome)
        genome_b64 = base64.b64encode(bytecode).decode('ascii')
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to compile genome: {e}")

    # Call worker to start the game
    try:
        result = await worker.execute(
            {
                "action": "start_game",
                "genome": genome_b64,
            }
        )
    except SimulationError as e:
        raise HTTPException(status_code=500, detail=f"Simulation error: {e}")

    # Check for worker errors
    if not result.get("success", True):
        raise HTTPException(status_code=500, detail=f"Worker error: {result.get('error', 'Unknown')}")

    # Get raw state for DB storage (worker needs this format)
    raw_state = result.get("state", {})
    winner = result.get("winner", -1)

    # Loop: If game starts with AI's turn (not player 0), make AI moves until player 0's turn
    max_ai_moves = 100  # Safety limit
    ai_moves_made = 0
    while raw_state.get("current_player", 0) != 0 and winner < 0 and ai_moves_made < max_ai_moves:
        # Get AI move
        try:
            ai_result = await worker.execute(
                {
                    "action": "get_ai_move",
                    "genome": genome_b64,
                    "state": raw_state,
                    "ai_type": "greedy",
                }
            )
        except SimulationError:
            break

        if not ai_result.get("success", True) or not ai_result.get("ai_move"):
            break

        ai_move_index = ai_result["ai_move"]["index"]

        # Apply AI move
        try:
            result = await worker.execute(
                {
                    "action": "apply_move",
                    "genome": genome_b64,
                    "state": raw_state,
                    "move_index": ai_move_index,
                }
            )
        except SimulationError:
            break

        if not result.get("success", True):
            break

        raw_state = result.get("state", {})
        winner = result.get("winner", -1)
        ai_moves_made += 1

    # Transform to frontend format for response
    frontend_state = _transform_worker_state(result)

    # Create session record with raw state
    session_id = str(uuid.uuid4())
    session = GameSession(
        id=session_id,
        game_id=game_id,
        session_id=ip_hash,  # Browser session identifier
        state_json=json.dumps(raw_state),
        version=1,
        completed=False,
    )
    db.add(session)

    # Increment play count
    game.play_count = (game.play_count or 0) + 1
    db.commit()

    return StartGameResponse(
        session_id=session_id,
        state=frontend_state,
        version=1,
        genome_name=game.id,
    )


@router.get("/sessions/{session_id}", response_model=SessionResponse)
async def get_session(
    session_id: str,
    db: SQLSession = Depends(get_db),
):
    """Get current session state for resuming a game.

    Returns the full session state including game_id, current state,
    version for optimistic locking, and completion status.
    """
    session = db.query(GameSession).filter(GameSession.id == session_id).first()
    if not session:
        raise HTTPException(status_code=404, detail="Session not found")

    # Parse stored state
    state = json.loads(session.state_json) if session.state_json else {}

    return SessionResponse(
        session_id=session.id,
        game_id=session.game_id,
        state=state,
        version=session.version,
        completed=session.completed,
    )


@router.post("/sessions/{session_id}/move", response_model=MoveResponse)
async def apply_move(
    session_id: str,
    move_request: MoveRequest,
    db: SQLSession = Depends(get_db),
    worker: SimulationWorker = Depends(get_worker),
):
    """Apply a move to the game session.

    Uses optimistic locking via the version field. If the provided version
    doesn't match the current server version, returns 409 Conflict.

    This prevents race conditions from duplicate requests or stale clients.
    """
    # Use pessimistic locking (SELECT FOR UPDATE) to prevent race conditions
    # between the version check and the update
    session = (
        db.query(GameSession)
        .filter(GameSession.id == session_id)
        .with_for_update()
        .first()
    )
    if not session:
        raise HTTPException(status_code=404, detail="Session not found")

    # Check if game is already completed
    if session.completed:
        raise HTTPException(status_code=400, detail="Game is already completed")

    # Optimistic locking check
    if move_request.version != session.version:
        raise HTTPException(
            status_code=409,
            detail=f"Version conflict: expected {session.version}, got {move_request.version}",
        )

    # Get current state
    current_state = json.loads(session.state_json) if session.state_json else {}

    # Get genome for the game
    game = db.query(Game).filter(Game.id == session.game_id).first()
    if not game:
        raise HTTPException(status_code=500, detail="Game configuration not found")

    # Compile genome to bytecode
    try:
        genome_dict = json.loads(game.genome_json)
        genome = genome_from_dict(genome_dict)
        bytecode = _bytecode_compiler.compile_genome(genome)
        genome_b64 = base64.b64encode(bytecode).decode('ascii')
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to compile genome: {e}")

    # Extract move index from move request
    # The frontend sends move with an 'index' field that corresponds to the worker's move index
    move_index = move_request.move.get("index", 0)

    # Call worker to apply the move
    try:
        result = await worker.execute(
            {
                "action": "apply_move",
                "genome": genome_b64,
                "state": current_state,
                "move_index": move_index,
            }
        )
    except SimulationError as e:
        raise HTTPException(status_code=500, detail=f"Simulation error: {e}")

    # Check for worker errors
    if not result.get("success", True):
        raise HTTPException(status_code=500, detail=f"Worker error: {result.get('error', 'Unknown')}")

    # Get raw state for DB storage (worker needs this format)
    raw_state = result.get("state", {})
    winner = result.get("winner", -1)

    # Loop: If it's AI's turn (not player 0), make AI moves until player 0's turn or game ends
    max_ai_moves = 100  # Safety limit
    ai_moves_made = 0
    ai_moves_list: list[dict[str, Any]] = []
    while raw_state.get("current_player", 0) != 0 and winner < 0 and ai_moves_made < max_ai_moves:
        # Get AI move
        try:
            ai_result = await worker.execute(
                {
                    "action": "get_ai_move",
                    "genome": genome_b64,
                    "state": raw_state,
                    "ai_type": "greedy",
                }
            )
        except SimulationError as e:
            # Check if game is actually over (empty hand win condition)
            winner = _check_win_from_state(raw_state)
            break

        if not ai_result.get("success", True) or not ai_result.get("ai_move"):
            # No moves available - check if game is actually over
            winner = _check_win_from_state(raw_state)
            break

        ai_move = ai_result["ai_move"]
        ai_move_index = ai_move["index"]

        # Track AI move for response
        ai_moves_list.append({
            "index": ai_move.get("index", 0),
            "type": ai_move.get("type", "unknown"),
            "label": ai_move.get("label", ""),
        })

        # Apply AI move
        try:
            result = await worker.execute(
                {
                    "action": "apply_move",
                    "genome": genome_b64,
                    "state": raw_state,
                    "move_index": ai_move_index,
                }
            )
        except SimulationError as e:
            # Check if game is actually over
            winner = _check_win_from_state(raw_state)
            break

        if not result.get("success", True):
            # Check if game is actually over
            winner = _check_win_from_state(raw_state)
            break

        raw_state = result.get("state", {})
        winner = result.get("winner", -1)
        ai_moves_made += 1

    # Final check: if winner still not determined, check from state
    if winner < 0:
        winner = _check_win_from_state(raw_state)

    # Transform to frontend format for response
    frontend_state = _transform_worker_state(result)

    # Include winner in frontend state
    frontend_state["winner"] = winner if winner >= 0 else None

    # Check completion
    is_completed = result.get("completed", False) or winner >= 0

    # Update session with raw state (for worker continuity)
    session.state_json = json.dumps(raw_state)
    session.version += 1
    session.completed = is_completed
    # Manually set updated_at since SQLAlchemy's onupdate only triggers on SQL UPDATE
    session.updated_at = datetime.now(timezone.utc)
    db.commit()

    return MoveResponse(
        state=frontend_state,
        version=session.version,
        completed=session.completed,
        ai_moves=ai_moves_list,
    )


@router.post("/sessions/{session_id}/flag", response_model=FlagResponse)
async def flag_session(
    session_id: str,
    flag_request: FlagRequest,
    db: SQLSession = Depends(get_db),
):
    """Flag a session as broken or problematic.

    This increments the flag_count on the associated game, which can
    trigger automatic demotion of consistently broken games.

    The flag_request.reason is accepted for future logging/analytics use
    but is not currently stored in the database.
    """
    session = db.query(GameSession).filter(GameSession.id == session_id).first()
    if not session:
        raise HTTPException(status_code=404, detail="Session not found")

    # Get the associated game
    game = db.query(Game).filter(Game.id == session.game_id).first()
    if game:
        # Increment flag count on the game
        # Note: flag_request.reason is available for logging if needed
        game.flag_count = (game.flag_count or 0) + 1
        db.commit()

    return FlagResponse(flagged=True)
