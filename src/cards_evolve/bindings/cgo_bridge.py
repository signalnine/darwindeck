"""Python wrapper for Go simulation engine via CGo."""

import ctypes
from pathlib import Path
import flatbuffers
from cards_evolve.bindings.cardsim import BatchResponse

# Load shared library
LIB_PATH = Path(__file__).parent.parent.parent.parent / "libcardsim.so"
_lib = ctypes.CDLL(str(LIB_PATH))

# Define C function signatures
_lib.SimulateBatch.argtypes = [ctypes.c_void_p, ctypes.c_int]
_lib.SimulateBatch.restype = ctypes.c_char_p
_lib.FreeCString.argtypes = [ctypes.c_char_p]
_lib.FreeCString.restype = None


def simulate_batch(batch_request_bytes: bytes) -> BatchResponse.BatchResponse:
    """Call Go simulation engine via CGo.

    Args:
        batch_request_bytes: Serialized BatchRequest flatbuffer

    Returns:
        BatchResponse object
    """
    # Call C function
    result_ptr = _lib.SimulateBatch(
        ctypes.c_char_p(batch_request_bytes),
        len(batch_request_bytes)
    )

    # Convert result to Python bytes
    result_bytes = ctypes.string_at(result_ptr)

    # Free C memory
    _lib.FreeCString(result_ptr)

    # Parse Flatbuffers response
    return BatchResponse.BatchResponse.GetRootAs(result_bytes, 0)
