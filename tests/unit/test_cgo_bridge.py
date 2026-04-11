"""Tests for the CGo bridge Python wrapper."""

import ctypes
from pathlib import Path

import pytest

LIB_PATH = Path(__file__).parent.parent.parent / "libcardsim.so"


@pytest.mark.skipif(not LIB_PATH.exists(), reason="libcardsim.so not built")
class TestCgoBridge:
    """Tests for the CGo shared library interface."""

    def test_free_response_exported(self) -> None:
        """FreeResponse symbol is exported and callable from libcardsim.so."""
        lib = ctypes.CDLL(str(LIB_PATH))
        lib.FreeResponse.argtypes = [ctypes.c_void_p]
        lib.FreeResponse.restype = None
        # Calling with NULL should not crash
        lib.FreeResponse(None)

    def test_simulate_batch_exported(self) -> None:
        """SimulateBatch symbol is exported from libcardsim.so."""
        lib = ctypes.CDLL(str(LIB_PATH))
        assert hasattr(lib, "SimulateBatch")
