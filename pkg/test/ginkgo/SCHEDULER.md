# Test Scheduler Design

## Overview

The OpenShift test suite uses a sophisticated scheduler to execute thousands of tests efficiently while managing resource constraints. This document describes the scheduling philosophy and implementation.

## Core Principles

### 1. Resource-Aware Parallelism

Tests have different resource requirements. Running too many resource-intensive tests concurrently can:
- Exhaust CPU on test nodes
- Cause cloud provider quota issues (storage, networking)
- Lead to test failures due to resource contention rather than actual bugs

**Solution**: Tests are organized into buckets with different parallelism levels based on their resource intensity.

### 2. Avoiding Long-Tail Problems

Traditional sequential bucket execution has a "long tail" problem:
```
Bucket A: [============================] (30 parallel)
          [============================]
          [============================]
          [========]                     <- Last test runs alone!

Bucket B:           Waiting...
```

When Bucket A has only one slow test remaining, we're stuck at parallelism=1, wasting cluster capacity.

**Solution**: Chained bucket execution - activate the next bucket when the current bucket's active tests drop below the next bucket's max parallelism.

### 3. Gradual Parallelism Transitions

When transitioning from high-parallelism to low-parallelism buckets, we must respect the lower limit:

**WRONG**:
```
Bucket A (max=30): 14 active tests
Bucket B (max=15): Activated
Total: 14 + 15 = 29 tests running  ❌ Violates B's limit!
```

**CORRECT**:
```
Bucket A (max=30): 14 active tests
Bucket B (max=15): Activated
Global max: min(30, 15) = 15
Total: 14 from A + 1 from B = 15  ✅
```

As A's tests complete, they're replaced by B's tests until B fully takes over at parallelism=15.

## Test Organization

### Size-Based Buckets

Tests are labeled with size tags indicating resource intensity:

- **Size:S** - Small/Simple
  - No pod creation
  - Simple API calls, CLI commands
  - Quick execution (seconds to low minutes)
  - Examples: `oc explain`, basic CRUD operations, status checks
  - **Parallelism**: 2x base (e.g., 60 if base is 30)

- **Size:M** - Medium (DEFAULT for unlabeled tests)
  - 1-2 pods
  - Basic services, simple deployments
  - Moderate complexity
  - Examples: Single pod apps, basic networking, simple builds
  - **Parallelism**: 1x base (e.g., 30)

- **Size:L** - Large/Complex
  - Many pods, image builds, stress tests
  - Complex workflows, multi-replica deployments
  - High resource usage
  - Examples: must-gather, complex builds, stress tests
  - **Parallelism**: 0.5x base (e.g., 15)

### Bucket Execution Order

Tests execute in this order:

1. **Early** - Setup/invariant tests (parallelism=base)
2. **KubernetesTests** - Upstream k8s tests including network (parallelism=base)
3. **StorageTests** - Storage-intensive tests (parallelism=base/2)
4. **NetworkTests** - OpenShift networking tests (parallelism=base/2)
5. **OpenShiftTests-S** - Small OpenShift tests (parallelism=base*2)
6. **OpenShiftTests-M** - Medium OpenShift tests (parallelism=base)
7. **OpenShiftTests-L** - Large OpenShift tests (parallelism=base/2)
8. **MustGatherTests** - Resource collection tests (parallelism=base)
9. **Late** - Cleanup/invariant tests (parallelism=base)

Early and Late buckets run standalone (not chained). Buckets 2-8 use chained execution.

## Chained Bucket Scheduler

### Algorithm

The `chainedBucketScheduler` implements the `TestScheduler` interface and manages multiple test buckets with progressive activation.

#### Key Components

1. **Test Queues**: One queue per bucket containing tests to execute
2. **Active Test Counts**: Tracks how many tests from each bucket are currently running
3. **Current Bucket Index**: The primary bucket we're executing from
4. **Global Max Parallelism**: Calculated as the minimum of all active buckets' max parallelism

#### Activation Logic

```go
func shouldActivateNextBucket() bool {
    currentActive := activeTestCounts[currentBucketIdx]
    nextBucketMax := buckets[currentBucketIdx + 1].MaxParallelism

    return currentActive < nextBucketMax
}
```

When bucket A has fewer active tests than bucket B's max parallelism, bucket B is activated.

#### Scheduling Logic

On each `GetNextTestToRun()` call:

1. **Calculate global max parallelism**
   ```go
   globalMax = currentBucket.MaxParallelism
   for each active bucket:
       globalMax = min(globalMax, bucket.MaxParallelism)
   ```

2. **Check global limit**
   ```go
   totalActive = sum of all activeTestCounts
   if totalActive >= globalMax:
       wait for a test to complete
   ```

3. **Try to get a test from any active bucket**
   - Iterate through active buckets (currentBucketIdx onwards)
   - Skip buckets that are empty or at their individual max
   - Find a runnable test (respecting conflicts/taints)
   - If found, increment that bucket's active count
   - Check if we should activate the next bucket

4. **Handle bucket transitions**
   - When current bucket is exhausted (empty queue + zero active):
     - End monitoring interval for that bucket
     - Advance to next bucket
     - Start monitoring interval for new current bucket

### Example Execution

Base parallelism = 30

**Phase 1: KubernetesTests (max=30)**
```
Active: 30 tests from Kube
Global max: 30
```

**Phase 2: Kube Tail (14 active tests)**
```
Kube active: 14
14 < 15 (Storage max) → Activate StorageTests
Global max: min(30, 15) = 15
Active: 14 from Kube, can add 1 from Storage
```

**Phase 3: Transition**
```
Kube active: 13 → 12 → 11... (completing)
Storage active: 2 → 3 → 4... (ramping up)
Total: Always ≤ 15
```

**Phase 4: Storage Dominates (max=15)**
```
Kube active: 0 (exhausted)
Storage active: 15
Global max: 15
```

**Phase 5: Storage Tail (7 active tests)**
```
Storage active: 7
7 < 15 (Network max) → Activate NetworkTests
Global max: min(15, 15) = 15
Active: 7 from Storage, can add 8 from Network
```

And so on through all buckets...

## Conflict and Taint Management

### Conflicts

Some tests cannot run concurrently because they:
- Modify the same cluster resource
- Depend on exclusive access to a feature
- Would interfere with each other's validation

Tests declare conflicts in their spec:
```go
Isolation.Conflict: ["egressip", "network-policy"]
```

The scheduler ensures no two tests with overlapping conflicts run simultaneously within the same conflict group.

### Taints and Tolerations

**Taints** allow tests to declare they modify cluster state in specific ways:
```go
Isolation.Taint: ["cluster-network-config"]
```

**Tolerations** allow tests to declare they can run despite certain cluster modifications:
```go
Isolation.Toleration: ["cluster-network-config"]
```

Only tests that tolerate all active taints can run. Tests without tolerations can only run when no taints are active.

### Serial Tests

Tests marked with `[Serial]` in their name run sequentially after all parallel tests complete. This is for tests that:
- Modify global cluster state
- Cannot tolerate any other tests running
- Need exclusive cluster access

## Worker Pool

Workers are goroutines that continuously:
1. Call `GetNextTestToRun()` (blocks until a test is available)
2. Execute the test
3. Call `MarkTestComplete()` (signals other workers)
4. Repeat until all tests are done

The number of workers equals the maximum parallelism (typically base parallelism, e.g., 30).

Workers don't need to know about buckets - the scheduler handles all bucket logic internally.

## Benefits

1. **Eliminates long-tail waste**: No bucket sits idle while another has one slow test
2. **Respects resource limits**: Each bucket's max parallelism is enforced globally
3. **Smooth transitions**: Gradual ramp-down/ramp-up between buckets
4. **Flexible**: Easy to add new buckets or adjust parallelism levels
5. **Observable**: Monitoring intervals track each bucket's execution time
6. **Conflict-aware**: Tests with conflicts are safely serialized
7. **Taint-aware**: Cluster state changes are properly managed

## Monitoring

Each bucket records a monitoring interval:
```
StartInterval("OpenShiftTests-S")
... tests execute ...
EndInterval()
Log: "Completed OpenShiftTests-S bucket in 15m32s"
```

This provides visibility into:
- How long each bucket takes
- When buckets overlap (chaining periods)
- Whether parallelism adjustments are working

## Future Improvements

Potential enhancements:

1. **Dynamic parallelism**: Adjust based on actual cluster resource usage
2. **Priority scheduling**: Run flaky/important tests first
3. **Smart bucket ordering**: Reorder based on historical runtimes
4. **Adaptive chaining threshold**: Trigger next bucket at different thresholds (e.g., 50% remaining)
5. **Cross-bucket conflict detection**: Allow conflicts to span buckets

## Implementation Files

- `pkg/test/ginkgo/chained_scheduler.go` - Chained bucket scheduler
- `pkg/test/ginkgo/queue.go` - Test queue and execution logic
- `pkg/test/ginkgo/cmd_runsuite.go` - Bucket definition and suite execution
- `test/extended/**/*.go` - Individual test files with Size labels
