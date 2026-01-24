# tests/unit/test_web_worker.py
"""Tests for SimulationWorker that manages Go subprocess."""

import os
from pathlib import Path

import pytest
from darwindeck.web.worker import SimulationWorker, SimulationError


# Skip all tests in this module if worker binary not built
_worker_path = Path(__file__).parent.parent.parent / "bin" / "gosim-worker"
pytestmark = pytest.mark.skipif(
    not _worker_path.exists(),
    reason=f"Worker binary not built at {_worker_path}. Run 'make build-worker' first.",
)


class TestSimulationWorker:
    def test_ping_returns_success(self):
        worker = SimulationWorker()
        try:
            result = worker.execute_sync({"action": "ping"})
            assert result["success"] is True
        finally:
            worker.shutdown()

    def test_invalid_action_returns_error(self):
        worker = SimulationWorker()
        try:
            result = worker.execute_sync({"action": "invalid"})
            assert result["success"] is False
            assert "unknown action" in result["error"].lower()
        finally:
            worker.shutdown()

    def test_worker_restarts_after_shutdown(self):
        worker = SimulationWorker()
        worker.shutdown()
        # Should auto-restart on next execute
        result = worker.execute_sync({"action": "ping"})
        assert result["success"] is True
        worker.shutdown()

    def test_execute_with_timeout_handles_slow_response(self):
        """Test that timeout parameter works correctly."""
        worker = SimulationWorker()
        try:
            # Ping should complete well within timeout
            result = worker.execute_sync({"action": "ping"}, timeout=1.0)
            assert result["success"] is True
        finally:
            worker.shutdown()

    def test_multiple_sequential_commands(self):
        """Test that worker handles multiple commands in sequence."""
        worker = SimulationWorker()
        try:
            for _ in range(5):
                result = worker.execute_sync({"action": "ping"})
                assert result["success"] is True
        finally:
            worker.shutdown()


class TestSimulationWorkerErrorHandling:
    def test_nonexistent_worker_path_raises_error(self):
        """Test that invalid worker path raises SimulationError."""
        worker = SimulationWorker(worker_path="/nonexistent/path/worker")
        with pytest.raises(SimulationError):
            worker.execute_sync({"action": "ping"})
        worker.shutdown()


class TestSimulationWorkerAsync:
    @pytest.mark.asyncio
    async def test_async_ping_returns_success(self):
        worker = SimulationWorker()
        try:
            result = await worker.execute({"action": "ping"})
            assert result["success"] is True
        finally:
            worker.shutdown()

    @pytest.mark.asyncio
    async def test_async_invalid_action_returns_error(self):
        worker = SimulationWorker()
        try:
            result = await worker.execute({"action": "invalid"})
            assert result["success"] is False
            assert "unknown action" in result["error"].lower()
        finally:
            worker.shutdown()
