# Python ↔ Golang Interface Decision

## Benchmark Results

- War game speedup: 2.9x
- Python overhead acceptable: Yes (Python 3.13 is surprisingly fast)
- Absolute performance: Python 0.07ms, Golang 0.03ms per game

## Option A: CGo Bindings

**Pros:**
- Lower latency (direct C calls)
- No serialization overhead
- Simpler deployment (single binary)
- Better for tight loops with frequent calls
- Minimal overhead per call (~100ns)

**Cons:**
- Tighter coupling
- CGo debugging is harder
- Must rebuild for each change
- Platform-specific builds required

**Implementation:**
```go
//export PlayWarGame
func PlayWarGame(seed int64, maxTurns int) int {
    // ...
}
```

**Python side:**
```python
from ctypes import CDLL, c_int64

libgosim = CDLL("./gosim.so")
result = libgosim.PlayWarGame(42, 1000)
```

## Option B: gRPC Service

**Pros:**
- Clean separation
- Language independence
- Easier debugging (wireshark)
- Can scale to distributed later
- Hot reload without Python restart

**Cons:**
- Serialization overhead (~1-2ms per call)
- More complex setup
- Requires proto definitions
- Network stack overhead even on localhost

**Implementation:**

proto:
```protobuf
service GameSimulator {
  rpc PlayWarGame(WarRequest) returns (WarResult);
}
```

## Decision

**Choice: CGo**

**Rationale:**

1. **Performance requirements:** Even though the speedup is only 2.9x currently, evolutionary algorithms will run millions of simulations. Every microsecond counts when multiplied by millions of iterations.

2. **Call frequency:** The simulation engine will be called in tight loops during fitness evaluation. CGo's ~100ns overhead is acceptable, while gRPC's ~1-2ms serialization overhead would dominate execution time.

3. **Development complexity:** For a single-machine evolutionary system, the added complexity of gRPC (proto definitions, service lifecycle, error handling) doesn't provide value. We're not building a distributed system (yet).

4. **Future scalability:** If we need distributed evolution later (parallel island GA), we can:
   - Keep CGo for local simulation
   - Add gRPC layer for coordinator ↔ worker communication
   - Best of both worlds

5. **Deployment simplicity:** Single Python package with bundled .so file is easier to distribute than Python + separate Go service.

## Next Steps

1. Create CGo wrapper for PlayWarGame
2. Build shared library (.so on Linux, .dylib on macOS, .dll on Windows)
3. Create Python ctypes bridge module
4. Add build scripts for cross-platform compilation
5. Benchmark with interface overhead to validate <10% overhead assumption

## Risk Mitigation

- **CGo debugging:** Use Go's built-in race detector, add extensive logging
- **Platform builds:** Document build process, consider GitHub Actions for CI
- **API versioning:** Use version tags in exported function names if API changes
