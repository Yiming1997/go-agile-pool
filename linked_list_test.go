package agilepool

import (
	"testing"
	"time"
)

// newTestWorker builds a worker with a fixed lastActiveAt for deterministic
// container tests. pool is nil because container operations never touch it.
// Shared by linked_list_test.go and min_heap_test.go (same package).
func newTestWorker(lastActive time.Time) *worker {
	return &worker{lastActiveAt: lastActive}
}

func TestLinkedListPopEmptyReturnsNil(t *testing.T) {
	ll := newLinkedList()
	if got := ll.Pop(); got != nil {
		t.Fatalf("Pop() on empty list = %v, want nil", got)
	}
	if ll.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", ll.Len())
	}
}

func TestLinkedListFIFOOrder(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(0, 0)
	w1 := newTestWorker(base)
	w2 := newTestWorker(base.Add(time.Second))
	w3 := newTestWorker(base.Add(2 * time.Second))

	ll.Add(w1)
	ll.Add(w2)
	ll.Add(w3)

	for i, want := range []*worker{w1, w2, w3} {
		if got := ll.Pop(); got != want {
			t.Fatalf("Pop() #%d = %p, want %p", i, got, want)
		}
	}
	if got := ll.Pop(); got != nil {
		t.Fatalf("Pop() after draining = %v, want nil", got)
	}
}

func TestLinkedListLen(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(0, 0)
	if ll.Len() != 0 {
		t.Fatalf("initial Len() = %d, want 0", ll.Len())
	}
	ll.Add(newTestWorker(base))
	ll.Add(newTestWorker(base))
	if ll.Len() != 2 {
		t.Fatalf("Len() after 2 Add = %d, want 2", ll.Len())
	}
	ll.Pop()
	if ll.Len() != 1 {
		t.Fatalf("Len() after Pop = %d, want 1", ll.Len())
	}
	ll.Pop()
	ll.Add(newTestWorker(base)) // drained, then refilled
	if ll.Len() != 1 {
		t.Fatalf("Len() after drain+Add = %d, want 1", ll.Len())
	}
}

func TestLinkedListRemoveExpiredBasic(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	// Expired iff lastActiveAt+expiry <= now. now = base+10s.
	now := base.Add(10 * time.Second)

	expired := newTestWorker(base) // base+1s <= now -> expired
	fresh := newTestWorker(now)    // now+1s  > now -> fresh
	ll.Add(expired)
	ll.Add(fresh)

	if removed := ll.RemoveExpired(now, expiry); removed != 1 {
		t.Fatalf("RemoveExpired removed %d, want 1", removed)
	}
	if ll.Len() != 1 {
		t.Fatalf("Len() after RemoveExpired = %d, want 1", ll.Len())
	}
	if got := ll.Pop(); got != fresh {
		t.Fatalf("survivor = %p, want fresh %p", got, fresh)
	}
}

// TestLinkedListRemoveExpiredFullTraversal verifies the core contract that
// distinguishes the list from the heap: insertion order is NOT monotonic with
// lastActiveAt, so RemoveExpired must traverse the whole list. A fresh worker
// is inserted BEFORE an old one; the old one (inserted later) must still be
// removed.
func TestLinkedListRemoveExpiredFullTraversal(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)

	fresh := newTestWorker(now) // inserted first, NOT expired
	old := newTestWorker(base)  // inserted second, expired
	ll.Add(fresh)
	ll.Add(old)

	if removed := ll.RemoveExpired(now, expiry); removed != 1 {
		t.Fatalf("removed %d, want 1 (must traverse past fresh head)", removed)
	}
	if got := ll.Pop(); got != fresh {
		t.Fatalf("survivor = %p, want fresh %p", got, fresh)
	}
}

// TestLinkedListRemoveExpiredPositions exercises removal at head, middle, and
// tail in one pass and asserts surviving order via Pop.
func TestLinkedListRemoveExpiredPositions(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)
	oldAt := base
	freshAt := now

	eHead := newTestWorker(oldAt)
	f1 := newTestWorker(freshAt)
	eMid := newTestWorker(oldAt)
	f2 := newTestWorker(freshAt)
	eTail := newTestWorker(oldAt)
	for _, w := range []*worker{eHead, f1, eMid, f2, eTail} {
		ll.Add(w)
	}

	if removed := ll.RemoveExpired(now, expiry); removed != 3 {
		t.Fatalf("removed %d, want 3", removed)
	}
	if ll.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", ll.Len())
	}
	if got := ll.Pop(); got != f1 {
		t.Fatalf("first survivor = %p, want f1 %p", got, f1)
	}
	if got := ll.Pop(); got != f2 {
		t.Fatalf("second survivor = %p, want f2 %p", got, f2)
	}
}

func TestLinkedListRemoveExpiredAll(t *testing.T) {
	ll := newLinkedList()
	base := time.Unix(1000, 0)
	expiry := 1 * time.Second
	now := base.Add(10 * time.Second)
	ll.Add(newTestWorker(base))
	ll.Add(newTestWorker(base.Add(time.Second)))

	if removed := ll.RemoveExpired(now, expiry); removed != 2 {
		t.Fatalf("removed %d, want 2", removed)
	}
	if ll.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", ll.Len())
	}
	if got := ll.Pop(); got != nil {
		t.Fatalf("Pop() after clearing all = %v, want nil", got)
	}
}
