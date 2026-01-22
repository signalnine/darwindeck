# tests/unit/test_web_security.py
"""Tests for web security utilities."""

import pytest
from unittest.mock import MagicMock
from darwindeck.web.security import get_real_ip, hash_ip, verify_admin
from fastapi import HTTPException


class TestGetRealIP:
    def test_returns_direct_ip_without_proxy(self):
        request = MagicMock()
        request.headers = {}
        request.client.host = "192.168.1.100"

        assert get_real_ip(request) == "192.168.1.100"

    def test_returns_forwarded_ip_with_proxy(self):
        request = MagicMock()
        request.headers = {"X-Forwarded-For": "203.0.113.50, 70.41.3.18"}
        request.client.host = "127.0.0.1"

        assert get_real_ip(request) == "203.0.113.50"

    def test_handles_single_forwarded_ip(self):
        request = MagicMock()
        request.headers = {"X-Forwarded-For": "203.0.113.50"}
        request.client.host = "127.0.0.1"

        assert get_real_ip(request) == "203.0.113.50"


class TestHashIP:
    def test_hash_is_consistent(self):
        h1 = hash_ip("192.168.1.1", salt="testsalt")
        h2 = hash_ip("192.168.1.1", salt="testsalt")
        assert h1 == h2

    def test_different_ips_have_different_hashes(self):
        h1 = hash_ip("192.168.1.1", salt="testsalt")
        h2 = hash_ip("192.168.1.2", salt="testsalt")
        assert h1 != h2

    def test_different_salts_produce_different_hashes(self):
        h1 = hash_ip("192.168.1.1", salt="salt1")
        h2 = hash_ip("192.168.1.1", salt="salt2")
        assert h1 != h2


class TestVerifyAdmin:
    @pytest.mark.asyncio
    async def test_allows_valid_api_key(self):
        request = MagicMock()
        request.client.host = "203.0.113.50"

        # Should not raise
        result = await verify_admin(request, api_key="correct-key", expected_key="correct-key")
        assert result is True

    @pytest.mark.asyncio
    async def test_allows_localhost(self):
        request = MagicMock()
        request.headers = {}
        request.client.host = "127.0.0.1"

        result = await verify_admin(request, api_key=None, expected_key="some-key")
        assert result is True

    @pytest.mark.asyncio
    async def test_rejects_invalid_key_from_remote(self):
        request = MagicMock()
        request.headers = {}
        request.client.host = "203.0.113.50"

        with pytest.raises(HTTPException) as exc:
            await verify_admin(request, api_key="wrong-key", expected_key="correct-key")
        assert exc.value.status_code == 403
