"""FastAPI application for DarwinDeck web UI."""

from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from slowapi import Limiter
from slowapi.middleware import SlowAPIMiddleware

from darwindeck.web.security import get_real_ip
from darwindeck.web.db import init_db, get_engine
from darwindeck.web.dependencies import get_worker


# Rate limiter using real IP
limiter = Limiter(key_func=get_real_ip)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    # Startup
    engine = get_engine()
    init_db(engine)
    worker = get_worker()

    yield

    # Shutdown
    worker.shutdown()


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(
        title="DarwinDeck Playtest",
        description="Play and rate evolved card games",
        version="0.1.0",
        lifespan=lifespan,
    )

    # CORS for frontend dev
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["http://localhost:5173"],  # Vite dev server
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Rate limiting
    app.state.limiter = limiter
    app.add_middleware(SlowAPIMiddleware)

    # Health check
    @app.get("/health")
    async def health():
        return {"status": "ok"}

    # TODO: Add route imports here
    # from darwindeck.web.routes import games, sessions, ratings, admin
    # app.include_router(games.router, prefix="/api/games", tags=["games"])

    return app


# Default app instance
app = create_app()
