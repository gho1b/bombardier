package bombardier

import (
	"math"
	"testing"
	"time"
)

func TestCouintingCompletionBarrierWait(t *testing.T) {
	parties := uint64(10)
	b := NewCountingCompletionBarrier(1000)
	for i := uint64(0); i < parties; i++ {
		go func() {
			for b.TryGrabWork() {
				b.JobDone()
			}
		}()
	}
	wc := make(chan struct{})
	go func() {
		<-b.Done()
		wc <- struct{}{}
	}()
	select {
	case <-wc:
		return
	case <-time.After(100 * time.Millisecond):
		t.Fail()
	}
}

func TestTimedCompletionBarrierWait(t *testing.T) {
	parties := uint64(10)
	duration := 100 * time.Millisecond
	timeout := duration * 2
	err := 15 * time.Millisecond
	sleepDuration := 2 * time.Millisecond
	b := NewTimedCompletionBarrier(duration)
	for i := uint64(0); i < parties; i++ {
		go func() {
			for b.TryGrabWork() {
				time.Sleep(sleepDuration)
				b.JobDone()
			}
		}()
	}
	wc := make(chan time.Duration)
	go func() {
		start := time.Now()
		<-b.Done()
		wc <- time.Since(start)
	}()
	select {
	case actual := <-wc:
		if !approximatelyEqual(duration, actual, sleepDuration+err) {
			t.Errorf("Expected to run %v, but ran %v instead", duration, actual)
		}
	case <-time.After(timeout):
		t.Error("Barrier hanged")
	}
}

func TestTimeBarrierCancel(t *testing.T) {
	b := NewTimedCompletionBarrier(9000 * time.Second)
	sleepTime := 100 * time.Millisecond
	go func() {
		time.Sleep(sleepTime)
		b.Cancel()
	}()
	select {
	case <-b.Done():
		if c := b.Completed(); c != 1.0 {
			t.Error(c)
		}
	case <-time.After(sleepTime * 2):
		t.Fail()
	}
}

func TestCountedBarrierCancel(t *testing.T) {
	parties := uint64(10)
	b := NewCountingCompletionBarrier(math.MaxUint64)
	sleepTime := 100 * time.Millisecond
	for i := uint64(0); i < parties; i++ {
		go func() {
			for b.TryGrabWork() {
				b.JobDone()
			}
		}()
	}
	go func() {
		time.Sleep(sleepTime)
		b.Cancel()
	}()
	select {
	case <-b.Done():
		if c := b.Completed(); c != 1.0 {
			t.Error(c)
		}
	case <-time.After(5 * time.Second):
		t.Fail()
	}
}

func TestTimeBarrierPanicOnBadDuration(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("shouldn't be empty")
			t.Fail()
		}
	}()
	NewTimedCompletionBarrier(-1 * time.Second)
	t.Error("unreachable")
	t.Fail()
}

func approximatelyEqual(expected, actual, err time.Duration) bool {
	return expected-err < actual && actual < expected+err
}
