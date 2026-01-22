"""Isolated Go worker subprocess for game simulation."""

from __future__ import annotations

import asyncio
import json
import select
import subprocess
import threading
from pathlib import Path
from typing import Any


class SimulationError(Exception):
    """Error during simulation execution."""

    pass


class SimulationWorker:
    """Manages an isolated Go worker subprocess for game simulation.

    The worker communicates via JSON over stdin/stdout. Crashes are
    isolated - if the worker dies, it's automatically restarted.

    Thread-safety: execute_sync() is protected by a threading.Lock.
    Async-safety: execute() is protected by an asyncio.Lock.

    Note: Timeout handling uses select.select() which only works on Unix-like
    systems. Windows is not supported (would require threading-based timeout).
    """

    def __init__(self, worker_path: str | None = None):
        """Initialize the worker manager.

        Args:
            worker_path: Path to gosim-worker binary. If None, uses default location.
        """
        if worker_path is None:
            # Default: bin/gosim-worker relative to project root
            project_root = Path(__file__).parent.parent.parent.parent
            worker_path = str(project_root / "bin" / "gosim-worker")

        self._worker_path = worker_path
        self._process: subprocess.Popen | None = None
        self._lock = threading.Lock()
        self._async_lock: asyncio.Lock | None = None

    def _start_worker(self) -> None:
        """Start or restart the Go worker subprocess.

        Raises:
            SimulationError: If worker binary cannot be started.
        """
        if self._process is not None:
            try:
                self._process.terminate()
                self._process.wait(timeout=1)
            except Exception:
                try:
                    self._process.kill()
                except Exception:
                    pass

        try:
            self._process = subprocess.Popen(
                [self._worker_path],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=1,
            )
        except FileNotFoundError as e:
            raise SimulationError(
                f"Worker binary not found: {self._worker_path}"
            ) from e
        except PermissionError as e:
            raise SimulationError(
                f"Worker binary not executable: {self._worker_path}"
            ) from e
        except OSError as e:
            raise SimulationError(f"Failed to start worker: {e}") from e

    def _ensure_running(self) -> None:
        """Ensure worker process is running."""
        if self._process is None or self._process.poll() is not None:
            self._start_worker()

    def _read_with_timeout(self, timeout: float) -> str:
        """Read a line from stdout with timeout.

        Args:
            timeout: Maximum seconds to wait for response

        Returns:
            The response line

        Raises:
            SimulationError: If timeout or read error occurs
        """
        assert self._process is not None
        assert self._process.stdout is not None

        stdout_fd = self._process.stdout.fileno()

        # Use select for timeout on Unix-like systems
        if hasattr(select, "select"):
            ready, _, _ = select.select([stdout_fd], [], [], timeout)
            if not ready:
                raise SimulationError(f"Worker timeout after {timeout}s")

        response_line = self._process.stdout.readline()
        if not response_line:
            raise SimulationError("Worker closed unexpectedly")

        return response_line

    def execute_sync(
        self, command: dict[str, Any], timeout: float = 5.0
    ) -> dict[str, Any]:
        """Execute command synchronously. Thread-safe.

        Args:
            command: JSON-serializable command dict
            timeout: Max wait time in seconds

        Returns:
            Response dict from worker

        Raises:
            SimulationError: If worker crashes or times out
        """
        with self._lock:
            self._ensure_running()
            assert self._process is not None
            assert self._process.stdin is not None
            assert self._process.stdout is not None

            try:
                # Write command
                self._process.stdin.write(json.dumps(command) + "\n")
                self._process.stdin.flush()

                # Read response with timeout
                response_line = self._read_with_timeout(timeout)

                return json.loads(response_line)

            except SimulationError:
                # Re-raise simulation errors as-is
                self._start_worker()
                raise

            except json.JSONDecodeError as e:
                # Worker returned invalid JSON
                self._start_worker()
                raise SimulationError(f"Invalid JSON from worker: {e}") from e

            except Exception as e:
                # Worker crashed - restart for next call
                self._start_worker()
                raise SimulationError(f"Worker error: {e}") from e

    async def execute(
        self, command: dict[str, Any], timeout: float = 5.0
    ) -> dict[str, Any]:
        """Execute command asynchronously. Async-safe.

        Args:
            command: JSON-serializable command dict
            timeout: Max wait time in seconds

        Returns:
            Response dict from worker

        Raises:
            SimulationError: If worker crashes or times out
        """
        if self._async_lock is None:
            self._async_lock = asyncio.Lock()

        async with self._async_lock:
            # Run sync version in thread pool
            loop = asyncio.get_event_loop()
            return await asyncio.wait_for(
                loop.run_in_executor(None, self.execute_sync, command, timeout),
                timeout=timeout + 1,  # Extra second for overhead
            )

    def shutdown(self) -> None:
        """Shutdown the worker process."""
        if self._process is not None:
            try:
                self._process.terminate()
                self._process.wait(timeout=2)
            except Exception:
                try:
                    self._process.kill()
                except Exception:
                    pass
            self._process = None

    def __del__(self) -> None:
        """Cleanup worker on garbage collection."""
        self.shutdown()
