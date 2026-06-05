package agilepool

import (
	"testing"
	"time"
)

// newTestWorker is defined in linked_list_test.go (same package).

func TestMinHeapPopEmptyReturnsNil(t *testing.T) {
	h := newMinHeap()
	if got := h.Pop(); got != nil {
		t.Fatalf("Pop() on empty heap = %v, want nil", got)
	}
	if h.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", h.Len())
	}
}

// TestMinHeapPopOrder verifies workers come out oldest-first regardless of
// insertion order.
func TestMinHeapPopOrder(t *testing.T) {
	h := newMinHeap()
	base := time.Unix(0, 0)
	w3 := newTestWorker(base.Add(3 * time.Second))
	w1 := newTestWorker(base.Add(1 * time.Second))
	w2 := newTestWorker(base.Add(2 * time.Second))

	h.Add(w3) // insert out of order
	h.Add(w1)
	h.Add(w2)

	for i, want := range []*worker{w1, w2, w3} {
		if got := h.Pop(); got != want {
			t.Fatalf("Pop() #%d = %p, want %p (oldest-first)", i, got, want)
		}
	}
	if got := h.Pop(); got != nil {
		t.Fatalf("Pop() after draining = %v, want nil", got)
	}
}

func TestMinHeapLen(t *testing.T) {
	h := newMinHeap()
	base := time.Unix(0, 0)
	if h.Len() != 0 {
		t.Fatalf("initial Len() = %d, want 0", h.Len())
	}
	h.Add(newTestWorker(base))
	h.Add(newTestWorker(base.Add(time.Second)))
	if h.Len() != 2 {
		t.Fatalf("Len() after 2 Add = %d, want 2", h.Len())
	}
	h.Pop()
	if h.Len() != 1 {
		t.Fatalf("Len() after Pop = %d, want 1", h.Len())
	}
}

// TestMinHeapRemoveExpiredEarlyStop verifies the O(k log n) optimization: when
// the root (oldest) is not expired, RemoveExpired removes nothing and stops.
func TestMinHeapRemoveExpiredEarlyStop(t *testing.T) {
	h := newMinHeap()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)

	h.Add(newTestWorker(now)) // root = now, not expired
	h.Add(newTestWorker(now.Add(time.Second)))

	if removed := h.RemoveExpired(now, expiry); removed != 0 {
		t.Fatalf("RemoveExpired removed %d, want 0 (fresh root)", removed)
	}
	if h.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", h.Len())
	}
}

// TestMinHeapRemoveExpiredPrefix verifies only the expired prefix (oldest
// workers) is removed and fresher workers survive.
func TestMinHeapRemoveExpiredPrefix(t *testing.T) {
	h := newMinHeap()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)

	old1 := newTestWorker(base)                  // expired
	old2 := newTestWorker(base.Add(time.Second)) // expired
	fresh := newTestWorker(now)                  // fresh
	h.Add(fresh)
	h.Add(old1)
	h.Add(old2)

	if removed := h.RemoveExpired(now, expiry); removed != 2 {
		t.Fatalf("removed %d, want 2", removed)
	}
	if h.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", h.Len())
	}
	if got := h.Pop(); got != fresh {
		t.Fatalf("survivor = %p, want fresh %p", got, fresh)
	}
}

func TestMinHeapRemoveExpiredAll(t *testing.T) {
	h := newMinHeap()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)
	h.Add(newTestWorker(base))
	h.Add(newTestWorker(base.Add(2 * time.Second)))

	if removed := h.RemoveExpired(now, expiry); removed != 2 {
		t.Fatalf("removed %d, want 2", removed)
	}
	if h.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", h.Len())
	}
	if got := h.Pop(); got != nil {
		t.Fatalf("Pop() after clearing = %v, want nil", got)
	}
}
