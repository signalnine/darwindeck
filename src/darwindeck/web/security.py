# src/darwindeck/web/security.py
"""Security utilities for web API."""

from __future__ import annotations

import hashlib
import hmac
import os
from typing import Optional

from fastapi import HTTPException, Request


def get_real_ip(request: Request) -> str:
    """Get real client IP, handling reverse proxy.

    Args:
        request: FastAPI request object

    Returns:
        Client IP address (first in X-Forwarded-For chain if present)
    """
    forwarded = request.headers.get("X-Forwarded-For")
    if forwarded:
        # X-Forwarded-For: client, proxy1, proxy2
        return forwarded.split(",")[0].strip()
    # Handle case where client is None (can happen behind some proxies)
    if request.client is None:
        return "0.0.0.0"
    return request.client.host


def hash_ip(ip: str, salt: Optional[str] = None) -> str:
    """Hash IP address for privacy-preserving storage.

    Uses HMAC-SHA256 with a salt to prevent rainbow table attacks.

    Args:
        ip: IP address to hash
        salt: Secret salt (defaults to DARWINDECK_IP_SALT env var)

    Returns:
        Hex-encoded hash
    """
    if salt is None:
        salt = os.environ.get("DARWINDECK_IP_SALT", "default-dev-salt")

    return hmac.new(
        salt.encode(),
        ip.encode(),
        hashlib.sha256
    ).hexdigest()[:32]  # Truncate for storage


async def verify_admin(
    request: Request,
    api_key: Optional[str],
    expected_key: Optional[str],
) -> bool:
    """Verify admin access via API key or localhost.

    Args:
        request: FastAPI request object
        api_key: Provided API key (from X-Admin-Key header)
        expected_key: Expected API key (from environment)

    Returns:
        True if authorized

    Raises:
        HTTPException: 403 if not authorized
    """
    # Option 1: Valid API key (use compare_digest to prevent timing attacks)
    if expected_key and api_key and hmac.compare_digest(api_key, expected_key):
        return True

    # Option 2: Localhost access
    real_ip = get_real_ip(request)
    if real_ip in ("127.0.0.1", "::1", "localhost"):
        return True

    raise HTTPException(status_code=403, detail="Admin access denied")
