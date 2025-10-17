# Bug Fixes Report

This document summarizes the potential bugs found during the comprehensive code review and the fixes applied.

## Critical Bugs Fixed

### 1. Resource Leak in Notification Worker (CRITICAL - FIXED)

**File:** `pkg/notify/notify.go`  
**Lines:** 387-415  
**Severity:** Critical

**Problem:**
The `sendWorker` function had `defer` statements inside an infinite loop:

```go
func (n *notifier) sendWorker(i int) {
    for {
        select {
        case work := <-n.channelSendWork:
            defer func() { telemetry.End(workCtx, nil) }()  // BUG!
            workCtx, cancel := context.WithTimeout(...)
            defer cancel()  // BUG!
            // ...
        }
    }
}
```

**Issue Details:**
- `defer` statements are executed when the function returns, NOT when the loop iteration ends
- Each iteration creates a new context with timeout but never cancels it until the worker goroutine exits
- This causes accumulation of:
  - Uncancelled contexts (memory leak)
  - Pending telemetry cleanup functions (memory leak)
  - Goroutines waiting on context timeouts (goroutine leak)

**Impact:**
- Memory usage grows continuously over time
- Goroutine count increases with each notification sent
- Eventually leads to OOM or performance degradation
- Production systems would crash under moderate load

**Fix:**
Extracted the loop body into a separate function `processSendWork()`:

```go
func (n *notifier) sendWorker(i int) {
    for {
        select {
        case <-n.Context().Done():
            return
        case work := <-n.channelSendWork:
            n.processSendWork(i, work)
        }
    }
}

func (n *notifier) processSendWork(i int, work sendWork) {
    workCtx := telemetry.StartWith(...)
    defer func() { telemetry.End(workCtx, nil) }()
    
    workCtx, cancel := context.WithTimeout(workCtx, 30*time.Second)
    defer cancel()
    // ...
}
```

Now `defer` statements are properly executed at the end of each iteration.

---

### 2. Weak Random Number Generation (MEDIUM - FIXED)

**Files:** 
- `pkg/api/api.go` (line 22)
- `pkg/util/time/time.go` (line 21)

**Severity:** Medium

**Problem:**
Using `math/rand` instead of `math/rand/v2`:

```go
import "math/rand"
// ...
feed.ID = rand.Uint64()  // Not properly seeded in Go 1.20+
```

**Issue Details:**
- In Go 1.20+, `math/rand` requires manual seeding for randomness
- Without seeding, the random number generator uses a fixed seed (1)
- This results in predictable "random" IDs across different runs
- Could lead to ID collisions when multiple instances start at the same time

**Impact:**
- Feed IDs might collide between different instances
- Security implications if IDs are used for any access control
- Reproducible IDs could be exploited

**Fix:**
Changed to use `math/rand/v2` which is automatically seeded:

```go
import "math/rand/v2"
```

The v2 package automatically seeds itself with a cryptographically random seed, eliminating the need for manual seeding.

---

## Potential Issues Identified (Not Fixed)

### 3. Potential Race Condition in Reload (LOW)

**File:** `pkg/notify/notify.go`  
**Lines:** 285-327  
**Severity:** Low

**Problem:**
In the `Reload` method, the old router and channel are closed before acquiring the write lock, but concurrent workers might still be using them with read locks:

```go
// Old router/channel still in use by workers with RLock
if err := n.router.Close(); err != nil {  // Close without lock
    log.Error(...)
}

n.mu.Lock()  // Lock acquired AFTER closing
n.router = router
n.mu.Unlock()
```

**Potential Impact:**
- Workers might call methods on closed router/channel
- Could result in "use of closed network connection" or similar errors
- Errors are logged, not fatal, so low severity

**Why Not Fixed:**
- Low severity - errors are logged and handled gracefully
- Requires architectural changes to the reload mechanism
- Current design minimizes critical section duration
- Reloads are infrequent operational events

**Recommendation:**
Consider implementing a "drain and replace" pattern where:
1. Mark old router/channel as "draining"
2. Wait for in-flight operations to complete
3. Then close and replace

---

### 4. Type Assertion Without Check (INFO)

**File:** `pkg/storage/feed/block/block.go`  
**Line:** 926  
**Severity:** Informational

**Code:**
```go
b.lastDataAccess.Load().(time.Time)
```

**Analysis:**
- Uses type assertion on atomic.Value without checking the type
- However, `lastDataAccess` is always initialized with `time.Time` at line 414
- The value is only stored as `time.Time` throughout the codebase
- Therefore, the assertion is safe in practice

**Recommendation:**
For better safety, consider using Go 1.19+'s typed atomic values:
```go
type atomicTime struct{ v atomic.Pointer[time.Time] }
```

However, this is not critical as the current code is functionally correct.

---

## Code Quality Observations

### Good Practices Found:
1. ✅ Consistent use of `defer` for resource cleanup
2. ✅ Proper HTTP response body closing with `defer resp.Body.Close()`
3. ✅ Context propagation throughout the codebase
4. ✅ Comprehensive error wrapping with context
5. ✅ Proper use of sync primitives (RWMutex, atomic, etc.)
6. ✅ Channel closing handled in single location (main.go)
7. ✅ Graceful shutdown with context cancellation

### Test Coverage:
- Mock implementations follow standard testify/mock patterns
- Intentional panics in mocks for early failure detection (correct design)

---

## Summary

**Bugs Fixed:** 2  
**Critical Issues:** 1 (Resource leak in notification worker)  
**Medium Issues:** 1 (Weak random number generation)  
**Potential Issues Noted:** 2 (Low severity, informational)

The most critical fix is the resource leak in the notification worker, which would cause production systems to crash under load. The random number generation fix improves security and prevents potential ID collisions.
