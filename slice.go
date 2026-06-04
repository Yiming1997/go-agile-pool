package agilepool

import (
	"sort"
	"sync/atomic"
	"time"
)

// Slice implements IdleWorkerContainer using a dynamic array (slice).
// Workers are stored in FIFO order by insertion time, matching the behavior
// of LinkedList. Add appends to the tail, Pop removes from the head.
// Not safe for concurrent use; the caller (Pool) serializes access via muIdle.
type Slice struct {
	workers []*worker
	length  int64
}

// Add appends a worker to the tail of the slice. O(1) amortized.
func (s *Slice) Add(w *worker) {
	s.workers = append(s.workers, w)
	atomic.AddInt64(&s.length, 1)
}

// Pop removes and returns the worker at the head of the slice (FIFO).
// Returns nil if the slice is empty. O(n) due to shifting elements.
func (s *Slice) Pop() *worker {
	if s.Len() == 0 {
		return nil
	}
	w := s.workers[0]
	s.workers[0] = nil
	s.workers = s.workers[1:]
	atomic.AddInt64(&s.length, -1)
	return w
}

// RemoveExpired removes all workers whose lastActiveAt + expiry <= now.
// Since workers are ordered by lastActiveAt (FIFO), expired workers are
// clustered at the front. Uses binary search to find the first non-expired
// worker. O(log n + k) where k is the number of expired workers removed.
func (s *Slice) RemoveExpired(now time.Time, expiry time.Duration) int {
	if s.Len() == 0 {
		return 0
	}

	cutoff := now.Add(-expiry)

	// Use sort.Search for binary search
	removed := sort.Search(len(s.workers), func(i int) bool {
		return s.workers[i].lastActiveAt.After(cutoff)
	})

	// Clear references to avoid memory leak
	for i := 0; i < removed; i++ {
		s.workers[i] = nil
	}

	if removed > 0 {
		s.workers = s.workers[removed:]
		atomic.AddInt64(&s.length, -int64(removed))
	}

	return removed
}

// Len returns the number of workers in the slice.
func (s *Slice) Len() int64 {
	return s.length
}

// newSlice creates a new empty Slice.
func newSlice() *Slice {
	return &Slice{
		workers: make([]*worker, 0),
	}
}
