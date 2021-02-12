package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type CompletionBarrier interface {
	Completed() float64
	TryGrabWork() bool
	JobDone()
	Done() <-chan struct{}
	Cancel()
}

type CountingCompletionBarrier struct {
	numReqs, reqsGrabbed, reqsDone uint64
	doneChan                       chan struct{}
	closeOnce                      sync.Once
}

func NewCountingCompletionBarrier(numReqs uint64) CompletionBarrier {
	c := new(CountingCompletionBarrier)
	c.reqsDone, c.reqsGrabbed, c.numReqs = 0, 0, numReqs
	c.doneChan = make(chan struct{})
	return CompletionBarrier(c)
}

func (c *CountingCompletionBarrier) TryGrabWork() bool {
	select {
	case <-c.doneChan:
		return false
	default:
		reqsDone := atomic.AddUint64(&c.reqsGrabbed, 1)
		return reqsDone <= c.numReqs
	}
}

func (c *CountingCompletionBarrier) JobDone() {
	reqsDone := atomic.AddUint64(&c.reqsDone, 1)
	if reqsDone == c.numReqs {
		c.closeOnce.Do(func() {
			close(c.doneChan)
		})
	}
}

func (c *CountingCompletionBarrier) Done() <-chan struct{} {
	return c.doneChan
}

func (c *CountingCompletionBarrier) Cancel() {
	c.closeOnce.Do(func() {
		close(c.doneChan)
	})
}

func (c *CountingCompletionBarrier) Completed() float64 {
	select {
	case <-c.doneChan:
		return 1.0
	default:
		reqsDone := atomic.LoadUint64(&c.reqsDone)
		return float64(reqsDone) / float64(c.numReqs)
	}
}

type TimedCompletionBarrier struct {
	doneChan  chan struct{}
	closeOnce sync.Once
	start     time.Time
	duration  time.Duration
}

func NewTimedCompletionBarrier(duration time.Duration) CompletionBarrier {
	if duration < 0 {
		panic("TimedCompletionBarrier: negative duration")
	}
	c := new(TimedCompletionBarrier)
	c.doneChan = make(chan struct{})
	c.start = time.Now()
	c.duration = duration
	go func() {
		time.AfterFunc(duration, func() {
			c.closeOnce.Do(func() {
				close(c.doneChan)
			})
		})
	}()
	return CompletionBarrier(c)
}

func (c *TimedCompletionBarrier) TryGrabWork() bool {
	select {
	case <-c.doneChan:
		return false
	default:
		return true
	}
}

func (c *TimedCompletionBarrier) JobDone() {
}

func (c *TimedCompletionBarrier) Done() <-chan struct{} {
	return c.doneChan
}

func (c *TimedCompletionBarrier) Cancel() {
	c.closeOnce.Do(func() {
		close(c.doneChan)
	})
}

func (c *TimedCompletionBarrier) Completed() float64 {
	select {
	case <-c.doneChan:
		return 1.0
	default:
		return float64(time.Since(c.start).Nanoseconds()) /
			float64(c.duration.Nanoseconds())
	}
}
