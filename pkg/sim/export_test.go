package sim

// Test-only bridges for the external sim_test package (which must live
// outside pkg/sim to import the real skeleton runners without a cycle).
var (
	// RunBatchSerialForTest exposes the serial golden-reference batch runner
	// so TestRunBatchMatchesSerialGolden (parallel_test.go) can assert the
	// parallel RunBatch is bit-identical to it. Permanent, not scaffolding.
	RunBatchSerialForTest = runBatchSerial

	// BatchWorkerCountForTest exposes the worker-bound computation so the
	// bounded-nested-parallelism contract (Wave I) stays pinned by test.
	BatchWorkerCountForTest = batchWorkerCount
)
