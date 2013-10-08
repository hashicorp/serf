package serf

import (
	"sync/atomic"
)

// LamportClock provides a thread safe implementation of a lamport clock
type LamportClock struct {
	counter uint64
}

type LamportTime uint64

// Time is used to return the current value of the lamport clock
func (l *LamportClock) Time() LamportTime {
	return LamportTime(atomic.LoadUint64(&l.counter))
}

// Increment is used to increment and return the value of the lamport clock
func (l *LamportClock) Increment() LamportTime {
	return LamportTime(atomic.AddUint64(&l.counter, 1))
}

// Witness is called to update our local clock if necessary after
// witnessing a clock value received from another process
func (l *LamportClock) Witness(v LamportTime) {
	// If the other value is old, we do not need to do anythin
	cur := atomic.LoadUint64(&l.counter)
	other := uint64(v)
	if other < cur {
		return
	}

	// Ensure that our local clock is at least one ahead
	atomic.CompareAndSwapUint64(&l.counter, cur, other+1)
}
