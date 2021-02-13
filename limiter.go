package bombardier

import (
	"math"
	"sync"
	"time"

	"github.com/juju/ratelimit"
)

type Token uint64

const (
	brk Token = iota
	cont
)

type Limiter interface {
	Pace(<-chan struct{}) Token
}

type Nooplimiter struct{}

func (n *Nooplimiter) Pace(<-chan struct{}) Token {
	return cont
}

type Bucketlimiter struct {
	limiter   *ratelimit.Bucket
	timerPool *sync.Pool
}

func NewBucketLimiter(rate uint64) Limiter {
	fillInterval, quantum := Estimate(rate, rateLimitInterval)
	return &Bucketlimiter{
		ratelimit.NewBucketWithQuantum(
			fillInterval, int64(quantum), int64(quantum),
		),
		&sync.Pool{
			New: func() interface{} {
				return time.NewTimer(math.MaxInt64)
			},
		},
	}
}

func (b *Bucketlimiter) Pace(done <-chan struct{}) (res Token) {
	wd := b.limiter.Take(1)
	if wd <= 0 {
		return cont
	}

	timer := b.timerPool.Get().(*time.Timer)
	timer.Reset(wd)
	select {
	case <-timer.C:
		res = cont
	case <-done:
		res = brk
	}
	b.timerPool.Put(timer)
	return
}
