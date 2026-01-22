# Web UI for Playing Evolved Games

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a web interface for playtesters to browse, play, and rate evolved card games, expanding the feedback pool beyond CLI users.

**Architecture:** Svelte frontend + Python FastAPI backend with isolated Go worker process for game simulation. SQLite for persistence.

**Tech Stack:** Svelte, TypeScript, Python, FastAPI, SQLite, Go (gosim)

---

## Overview

DarwinDeck evolves novel card games, but fitness evaluation is bottlenecked by human feedback. This web UI enables playtesters to:
- Browse evolved games with filtering and leaderboard
- Play games against AI opponents (random/greedy/MCTS)
- Rate games and flag broken mechanics
- Share links to specific games

Ratings feed back into the evolution fitness function, closing the loop.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Svelte Frontend                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Browser  │ │  Game    │ │  Rating  │ │  Leaderboard  │  │
│  │   View   │ │  Board   │ │   Form   │ │     View      │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
└───────────────────────┬─────────────────────────────────────┘
                        │ REST API + CSRF Token
┌───────────────────────┴─────────────────────────────────────┐
│                   Python FastAPI                            │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │  Genome  │ │ Rulebook │ │ Ratings  │ │    Game       │  │
│  │  Loader  │ │Generator │ │   API    │ │   Session     │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────┬───────┘  │
│                                                  │ IPC      │
│  ┌───────────────────────────────────────────────┴───────┐  │
│  │              Go Worker Process (isolated)             │  │
│  │   Subprocess running gosim, communicates via stdin/   │  │
│  │   stdout JSON. Crashes don't kill FastAPI server.     │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │                    SQLite (WAL mode)                    ││
│  │   Games table, Ratings table, Sessions table           ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

**Key decisions:**
- **Svelte + TypeScript** - Native animations for cards, reactive stores for game state
- **Isolated Go worker** - Subprocess for gosim, crashes are recoverable (not CGo in-process)
- **FastAPI** - Python handles genome loading, rulebook generation, ratings CRUD
- **SQLite** - Simple, embedded, WAL mode for concurrent access
- **Session-based identity** - Cookie-based sessions with IP tracking for abuse prevention

## Security Measures

### CGo Crash Isolation (Critical Fix)

**Problem:** Direct CGo calls mean a Go crash (segfault, panic) terminates the entire Python process.

**Solution:** Run gosim as a separate subprocess communicating via JSON over stdin/stdout:

```python
# web/simulation_worker.py
class SimulationWorker:
    def __init__(self):
        self.process = None
        self._start_worker()

    def _start_worker(self):
        """Start or restart the Go worker subprocess."""
        self.process = subprocess.Popen(
            ["./gosim-worker"],  # Compiled Go binary
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

    async def execute(self, command: dict, timeout: float = 5.0) -> dict:
        """Send command to worker, get response. Restart on crash.

        Note: Uses asyncio.Lock to prevent concurrent requests from
        corrupting the stdin/stdout pipes.
        """
        async with self._lock:  # Serialize access to subprocess
            try:
                # Write JSON command
                self.process.stdin.write(json.dumps(command).encode() + b'\n')
                self.process.stdin.flush()

                # Read JSON response with timeout
                response_line = await asyncio.wait_for(
                    asyncio.to_thread(self.process.stdout.readline),
                    timeout=timeout
                )
                return json.loads(response_line)
            except Exception as e:
                # Worker crashed - restart and report error
                self._start_worker()
                raise SimulationError(f"Worker crashed: {e}")
```

**Go worker binary:** Simple wrapper that reads JSON commands from stdin, calls gosim, writes JSON responses to stdout. Crashes are isolated.

### Rating Abuse Prevention (Critical Fix)

**Problem:** Anonymous sessions can be trivially reset to spam ratings.

**Solution:** Multi-layer defense:

```python
# Rate limiting per IP (using slowapi)
# Note: Use X-Forwarded-For header when behind reverse proxy (Caddy)
def get_real_ip(request: Request) -> str:
    """Get real client IP, handling reverse proxy."""
    forwarded = request.headers.get("X-Forwarded-For")
    if forwarded:
        return forwarded.split(",")[0].strip()
    return request.client.host

@app.post("/api/games/{id}/rate")
@limiter.limit("10/hour", key_func=get_real_ip)  # Max 10 ratings per IP per hour
async def rate_game(id: str, rating: RatingInput, request: Request):
    ...

# Additional checks in rating submission
async def submit_rating(game_id: str, session: Session, rating: int):
    # 1. Must have played the game (session has game_session record)
    played = await db.query(GameSession).filter(
        GameSession.game_id == game_id,
        GameSession.session_id == session.id,
        GameSession.completed == True
    ).first()
    if not played:
        raise HTTPException(400, "Must complete game before rating")

    # 2. Minimum play time (prevent speed-running for ratings)
    if played.duration_seconds < 30:
        raise HTTPException(400, "Game too short to rate")

    # 3. Track IP alongside session
    rating_record = Rating(
        game_id=game_id,
        session_id=session.id,
        ip_hash=hash_ip(request.client.host),  # For Sybil detection
        rating=rating,
    )
```

**CSRF Protection:**

```python
from fastapi_csrf_protect import CsrfProtect

@app.post("/api/games/{id}/rate")
async def rate_game(request: Request, csrf_protect: CsrfProtect = Depends()):
    await csrf_protect.validate_csrf(request)
    ...
```

### Admin Endpoint Security (Critical Fix)

**Problem:** `/api/admin/*` routes exposed without authentication.

**Solution:** API key authentication + localhost restriction:

```python
# Admin routes require API key header
ADMIN_API_KEY = os.environ.get("DARWINDECK_ADMIN_KEY")

async def verify_admin(request: Request, x_admin_key: str = Header(None)):
    # Option 1: API key from environment
    if ADMIN_API_KEY and x_admin_key == ADMIN_API_KEY:
        return True

    # Option 2: Localhost only (for CLI usage)
    # Note: Use get_real_ip() when behind reverse proxy
    real_ip = get_real_ip(request)
    if real_ip in ("127.0.0.1", "::1"):
        return True

    raise HTTPException(403, "Admin access denied")

@app.post("/api/admin/import")
async def import_genome(path: str, _: bool = Depends(verify_admin)):
    ...

@app.post("/api/admin/sync")
async def sync_genomes(_: bool = Depends(verify_admin)):
    ...
```

## Frontend Components

```
web/src/
├── routes/
│   ├── +page.svelte              # Home / Game browser
│   ├── game/[id]/+page.svelte    # Game play screen
│   └── leaderboard/+page.svelte
├── lib/
│   ├── components/
│   │   ├── Card.svelte           # Animated card with suit colors
│   │   ├── Hand.svelte           # Player's hand (clickable cards)
│   │   ├── GameBoard.svelte      # Main game area
│   │   ├── MoveButtons.svelte    # Legal actions (Pass, Fold, etc.)
│   │   ├── RuleSummary.svelte    # AI-generated 2-3 sentence summary
│   │   ├── RuleHint.svelte       # Contextual tooltip/modal
│   │   └── RatingForm.svelte     # Post-game feedback
│   └── stores/
│       ├── gameState.ts          # Reactive game state from server
│       └── session.ts            # Session token management
```

**Data Flow (single turn):**
1. Player clicks card → Svelte sends `POST /api/sessions/{id}/move` with CSRF token
2. FastAPI validates, sends command to Go worker subprocess
3. Go worker applies move, runs AI response, returns new state
4. FastAPI returns JSON game state with new version number
5. Svelte store updates → UI re-renders with animations

**Card Animation:** Svelte's `animate:flip` directive handles card movement with 300ms AI "thinking" delay.

## API Endpoints

```
Games & Browser
───────────────
GET  /api/games                    # List games (paginated, filterable)
     ?sort=rating|newest|random
     ?min_fitness=0.5
GET  /api/games/{id}               # Get game details + rulebook
GET  /api/games/{id}/summary       # AI-generated 2-3 sentence summary

Game Sessions
─────────────
POST /api/games/{id}/start         # Create new game session
     → { session_id, initial_state, legal_moves, version }

POST /api/sessions/{id}/move       # Apply player move
     { move_index: int, version: int }  # Optimistic locking
     → { state, legal_moves, ai_move?, game_over?, winner?, version }

GET  /api/sessions/{id}            # Resume in-progress game
POST /api/sessions/{id}/flag       # Flag as broken (rate limited)

Ratings & Feedback (Rate Limited + CSRF Protected)
──────────────────
POST /api/games/{id}/rate          # Submit rating (10/hour/IP)
     { rating: 1-5, comment?, felt_broken: bool }

GET  /api/leaderboard              # Top-rated games
     ?limit=20&offset=0

Admin (API Key or Localhost Only)
─────────────────────────────────
POST /api/admin/import             # Import genome from file
     X-Admin-Key: <secret>
POST /api/admin/sync               # Sync from evolution output dir
     X-Admin-Key: <secret>
```

## Database Schema

```sql
-- Evolved games imported from evolution output
CREATE TABLE games (
    id TEXT PRIMARY KEY,           -- genome_id (e.g., "GreenJack")
    genome_json TEXT NOT NULL,     -- Full genome for simulation
    rulebook_md TEXT,              -- Generated rulebook markdown (sanitized)
    summary TEXT,                  -- AI 2-3 sentence summary (sanitized)
    fitness REAL,                  -- From evolution (for filtering)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    play_count INTEGER DEFAULT 0,
    flag_count INTEGER DEFAULT 0,  -- "broken" flags for demotion
    status TEXT DEFAULT 'active'   -- active|demoted|archived
);

-- Player ratings (one per session per game)
CREATE TABLE ratings (
    id INTEGER PRIMARY KEY,
    game_id TEXT REFERENCES games(id),
    session_id TEXT,               -- Anonymous session
    ip_hash TEXT,                  -- Hashed IP for Sybil detection
    rating INTEGER CHECK(rating BETWEEN 1 AND 5),
    comment TEXT,                  -- HTML-escaped on read
    felt_broken BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(game_id, session_id)
);

-- In-progress game sessions (ephemeral, with optimistic locking)
CREATE TABLE game_sessions (
    id TEXT PRIMARY KEY,           -- UUID
    game_id TEXT REFERENCES games(id),
    session_id TEXT,               -- Player's browser session
    state_json TEXT,               -- Current GameState
    version INTEGER DEFAULT 1,     -- For optimistic locking
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    completed BOOLEAN DEFAULT FALSE,
    duration_seconds INTEGER       -- For rating validation
);

-- Browser sessions (anonymous identity)
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,           -- Cookie value
    ip_hash TEXT,                  -- For abuse detection
    created_at TIMESTAMP,
    last_seen TIMESTAMP,
    expires_at TIMESTAMP,          -- TTL for cleanup
    games_played INTEGER DEFAULT 0,
    games_rated INTEGER DEFAULT 0
);

-- Indexes
CREATE INDEX idx_games_status_fitness ON games(status, fitness DESC);
CREATE INDEX idx_ratings_game ON ratings(game_id);
CREATE INDEX idx_game_sessions_session ON game_sessions(session_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
```

## Race Condition Prevention

**Optimistic locking for game state:**

```python
async def apply_move(session_id: str, move_index: int, client_version: int):
    game_session = await db.query(GameSession).filter(
        GameSession.id == session_id
    ).first()

    # Check version matches
    if game_session.version != client_version:
        raise HTTPException(409, "State changed. Refresh and retry.")

    # Apply move via worker
    new_state = await worker.execute({
        "command": "apply_move",
        "state": game_session.state_json,
        "move_index": move_index,
    })

    # Update with incremented version
    game_session.state_json = new_state
    game_session.version += 1
    game_session.updated_at = datetime.utcnow()
    await db.commit()

    return {"state": new_state, "version": game_session.version}
```

## Session Cleanup

**Background task for expired data:**

```python
# Run on startup and every hour
@app.on_event("startup")
@repeat_every(seconds=3600)
async def cleanup_expired_sessions():
    now = datetime.utcnow()

    # Delete expired browser sessions (30 day TTL)
    await db.execute(
        delete(Session).where(Session.expires_at < now)
    )

    # Delete abandoned game sessions (24 hour TTL)
    cutoff = now - timedelta(hours=24)
    await db.execute(
        delete(GameSession).where(
            GameSession.updated_at < cutoff,
            GameSession.completed == False
        )
    )

    await db.commit()
```

## Input Sanitization

**All user-generated and AI-generated content is sanitized:**

```python
import bleach

def sanitize_markdown(md: str) -> str:
    """Sanitize markdown/HTML to prevent XSS."""
    return bleach.clean(
        md,
        tags=['p', 'strong', 'em', 'ul', 'ol', 'li', 'code', 'pre', 'h1', 'h2', 'h3'],
        attributes={},
        strip=True
    )

# On game import
game.rulebook_md = sanitize_markdown(generate_rulebook(genome))
game.summary = sanitize_markdown(generate_summary(genome))

# On rating display
rating.comment = bleach.clean(rating.comment, tags=[], strip=True)
```

## Game Publishing Flow

**Validation gates (all must pass):**
1. Syntax validation - Genome parses without errors
2. Headless crash test - Run 5 games with random AI in subprocess, no crashes
3. Fitness threshold - Default ≥0.4 (permissive to start)

**Probabilistic serving (solves cold-start):**
- 70% - High-confidence games (play_count >= 3)
- 30% - Exploration (new/unrated games)

**Demotion logic:**
```python
if game.flag_count >= 5 and game.flag_count / game.play_count > 0.3:
    game.status = 'demoted'
```

**CLI import for outliers:**
```bash
# From localhost (no API key needed)
uv run python -m darwindeck.cli.web import path/to/genome.json --skip-fitness-check

# Remote (requires API key)
curl -X POST https://example.com/api/admin/import \
  -H "X-Admin-Key: $ADMIN_KEY" \
  -d '{"path": "genome.json"}'
```

## Rule Teaching UI

**Layered learning:**

1. **Game start modal** - AI-generated 2-3 sentence summary (sanitized)
2. **During gameplay:**
   - Legal move highlighting (playable cards glow)
   - Phase indicator ("Discard Phase" / "Betting Round")
   - Contextual tooltips on hover
   - Win condition reminder
3. **Full rulebook** - Accessible via "?" button (sanitized markdown)

**Tooltip examples:**
- "✓ Playable - Matches discard pile (♠)"
- "✗ Cannot play - Must match suit or rank"

## Error Handling

| Error | Response | User Experience |
|-------|----------|-----------------|
| Invalid move | 400 | Toast: "Invalid move: must match suit" |
| Version conflict | 409 | Auto-refresh state, retry |
| Session expired | 410 | Modal: "Session expired. Start new game?" |
| Worker timeout | 504 | Spinner + auto-retry (3 attempts) |
| Worker crash | 500 | "Game error. Reported." + auto-flag + worker restart |
| Rate limited | 429 | "Slow down! Try again later." |

## Project Structure

```
darwindeck/
├── src/darwindeck/
│   ├── web/                  # NEW: Web UI backend
│   │   ├── app.py            # FastAPI app with security middleware
│   │   ├── routes/
│   │   │   ├── games.py
│   │   │   ├── sessions.py
│   │   │   └── ratings.py
│   │   ├── models.py         # SQLAlchemy models
│   │   ├── db.py
│   │   ├── worker.py         # Go subprocess manager
│   │   └── security.py       # Rate limiting, CSRF, admin auth
│   └── cli/
│       └── web.py            # CLI for web server
├── src/gosim/
│   └── cmd/worker/           # NEW: Standalone worker binary
│       └── main.go           # stdin/stdout JSON interface
├── web/                      # NEW: Svelte frontend
│   ├── src/
│   │   ├── routes/
│   │   └── lib/
│   ├── package.json
│   └── svelte.config.js
└── data/
    └── playtest.db           # SQLite database
```

## Development Commands

```bash
# Build Go worker
cd src/gosim && go build -o ../../bin/gosim-worker ./cmd/worker

# Backend
uv run python -m darwindeck.cli.web serve --reload

# Frontend
cd web && npm run dev

# Import games (localhost)
uv run python -m darwindeck.cli.web sync output/2026-01-21/

# Production
cd web && npm run build
DARWINDECK_ADMIN_KEY=secret uv run python -m darwindeck.cli.web serve --static web/build
```

## Deployment

Single VPS with Caddy reverse proxy:
- FastAPI + Uvicorn serves API and static Svelte build
- Go worker subprocess managed by FastAPI
- SQLite file at /data/playtest.db
- HTTPS via Caddy auto-certificates
- Environment: `DARWINDECK_ADMIN_KEY` for remote admin access

## Testing Strategy

```
tests/
├── unit/
│   ├── test_api_games.py
│   ├── test_api_sessions.py
│   ├── test_ratings.py
│   ├── test_security.py       # Rate limiting, CSRF, admin auth
│   └── test_sanitization.py   # XSS prevention
├── integration/
│   ├── test_full_game.py
│   ├── test_worker_crash.py   # Verify crash isolation
│   └── test_worker_restart.py
├── fuzz/
│   └── test_genome_fuzzing.py # Random genomes through worker
└── e2e/playwright/
    ├── browse_and_play.spec.ts
    └── rate_game.spec.ts
```

## Future Enhancements

- Multiplayer (human vs human) via WebSocket
- User accounts for persistent history
- Achievements and tournaments
- Mobile-optimized touch interactions
- PostgreSQL migration if SQLite becomes bottleneck
