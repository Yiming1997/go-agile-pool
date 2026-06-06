package agilepool

import (
	"math"
	"testing"
	"time"
)

func TestNewConfigDefaults(t *testing.T) {
	c := NewConfig()
	if c.cleanPeriod != 500*time.Millisecond {
		t.Errorf("cleanPeriod = %v, want 500ms", c.cleanPeriod)
	}
	if c.taskQueueSize != 10000 {
		t.Errorf("taskQueueSize = %d, want 10000", c.taskQueueSize)
	}
	if c.workerNumCapacity != math.MaxInt64 {
		t.Errorf("workerNumCapacity = %d, want MaxInt64", c.workerNumCapacity)
	}
	if c.workMode != BLOCK {
		t.Errorf("workMode = %d, want BLOCK", c.workMode)
	}
	if c.idleContainerType != LinkedListType {
		t.Errorf("idleContainerType = %d, want LinkedListType", c.idleContainerType)
	}
}

func TestWithOptionsSetValues(t *testing.T) {
	c := NewConfig(
		WithCleanPeriod(500*time.Millisecond),
		WithTaskQueueSize(42),
		WithWorkerNumCapacity(99),
		WithBlockMode(NONBLOCK),
		WithIdleContainerType(MinHeapType),
	)
	if c.cleanPeriod != 500*time.Millisecond {
		t.Errorf("cleanPeriod = %v, want 500ms", c.cleanPeriod)
	}
	if c.taskQueueSize != 42 {
		t.Errorf("taskQueueSize = %d, want 42", c.taskQueueSize)
	}
	if c.workerNumCapacity != 99 {
		t.Errorf("workerNumCapacity = %d, want 99", c.workerNumCapacity)
	}
	if c.workMode != NONBLOCK {
		t.Errorf("workMode = %d, want NONBLOCK", c.workMode)
	}
	if c.idleContainerType != MinHeapType {
		t.Errorf("idleContainerType = %d, want MinHeapType", c.idleContainerType)
	}
}

// TestWithGuardsIgnoreNonPositive verifies the >0 guards on cleanPeriod,
// taskQueueSize, and workerNumCapacity: zero/negative inputs are ignored and
// defaults preserved. WithBlockMode/WithIdleContainerType have no guard and
// are covered by TestWithOptionsSetValues.
func TestWithGuardsIgnoreNonPositive(t *testing.T) {
	c := NewConfig(
		WithCleanPeriod(0),
		WithTaskQueueSize(0),
		WithWorkerNumCapacity(-5),
	)
	if c.cleanPeriod != 500*time.Millisecond {
		t.Errorf("cleanPeriod = %v, want default 500ms (zero ignored)", c.cleanPeriod)
	}
	if c.taskQueueSize != 10000 {
		t.Errorf("taskQueueSize = %d, want default 10000 (zero ignored)", c.taskQueueSize)
	}
	if c.workerNumCapacity != math.MaxInt64 {
		t.Errorf("workerNumCapacity = %d, want default MaxInt64 (negative ignored)", c.workerNumCapacity)
	}
}
