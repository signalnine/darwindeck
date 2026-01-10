"""Python wrapper for Go simulation engine via CGo."""

import ctypes
from pathlib import Path
import flatbuffers
from darwindeck.bindings.cardsim import BatchResponse

# Load shared library
LIB_PATH = Path(__file__).parent.parent.parent.parent / "libcardsim.so"
_lib = ctypes.CDLL(str(LIB_PATH))

# Define C function signatures
_lib.SimulateBatch.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
_lib.SimulateBatch.restype = ctypes.c_void_p


def simulate_batch(batch_request_bytes: bytes) -> BatchResponse.BatchResponse:
    """Call Go simulation engine via CGo.

    Args:
        batch_request_bytes: Serialized BatchRequest flatbuffer

    Returns:
        BatchResponse object
    """
    # Create buffer from bytes
    buf = ctypes.create_string_buffer(batch_request_bytes)

    # Prepare output length
    response_len = ctypes.c_int()

    # Call C function
    result_ptr = _lib.SimulateBatch(
        ctypes.cast(buf, ctypes.c_void_p),
        len(batch_request_bytes),
        ctypes.byref(response_len)
    )

    # Copy result bytes
    result_bytes = bytes(ctypes.string_at(result_ptr, response_len.value))

    # Parse Flatbuffers response
    return BatchResponse.BatchResponse.GetRootAsBatchResponse(result_bytes, 0)
