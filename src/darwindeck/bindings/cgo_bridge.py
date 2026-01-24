"""Python wrapper for Go simulation engine via CGo."""

import ctypes
import sys
from pathlib import Path
import flatbuffers

# Add bindings directory to sys.path so FlatBuffers-generated code can import 'cardsim'
# The FlatBuffers compiler generates imports like 'from cardsim.X import X'
# rather than 'from darwindeck.bindings.cardsim.X import X'
_bindings_dir = str(Path(__file__).parent)
if _bindings_dir not in sys.path:
    sys.path.insert(0, _bindings_dir)

from darwindeck.bindings.cardsim import BatchResponse

# Load shared library
LIB_PATH = Path(__file__).parent.parent.parent.parent / "libcardsim.so"
_lib = ctypes.CDLL(str(LIB_PATH))

# Define C function signatures
_lib.SimulateBatch.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
_lib.SimulateBatch.restype = ctypes.c_void_p

# FreeResponse frees memory allocated by SimulateBatch
_lib.FreeResponse.argtypes = [ctypes.c_void_p]
_lib.FreeResponse.restype = None


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

    # Free the C memory allocated by Go
    _lib.FreeResponse(result_ptr)

    # Parse Flatbuffers response
    return BatchResponse.BatchResponse.GetRootAsBatchResponse(result_bytes, 0)
