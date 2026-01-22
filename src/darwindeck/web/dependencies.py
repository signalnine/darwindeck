"""FastAPI dependency injection."""

from __future__ import annotations

import os
from typing import Generator

from fastapi import Header, Request
from sqlalchemy.orm import Session as SQLSession

from darwindeck.web.db import get_session
from darwindeck.web.worker import SimulationWorker
from darwindeck.web.security import verify_admin as _verify_admin


# Singleton worker instance
_worker: SimulationWorker | None = None


def get_worker() -> SimulationWorker:
    """Get the simulation worker singleton."""
    global _worker
    if _worker is None:
        _worker = SimulationWorker()
    return _worker


def get_db() -> Generator[SQLSession, None, None]:
    """Get database session."""
    db = get_session()
    try:
        yield db
    finally:
        db.close()


async def verify_admin_dependency(
    request: Request,
    x_admin_key: str | None = Header(None),
) -> bool:
    """Dependency for admin-only routes."""
    expected_key = os.environ.get("DARWINDECK_ADMIN_KEY")
    return await _verify_admin(request, x_admin_key, expected_key)
