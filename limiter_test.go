package bombardier

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const maxRps = 10000000

func TestNoopLimiter(t *testing.T) {
	var lim Limiter = &Nooplimiter{}
	done := make(chan struct{})
	counter := uint64(0)
	var wg sync.WaitGroup
	wg.Add(int(defaultNumberOfConns))
	for i := uint64(0); i < defaultNumberOfConns; i++ {
		go func() {
			defer wg.Done()
			for {
				res := lim.Pace(done)
				if res != cont {
					t.Error("Nooplimiter should always return cont")
				}
				atomic.AddUint64(&counter, 1)
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
	if counter == 0 {
		t.Error("no events happened")
	}
}

func BenchmarkNoopLimiter(bm *testing.B) {
	var lim Limiter = &Nooplimiter{}
	done := make(chan struct{})
	bm.SetParallelism(int(defaultNumberOfConns) / runtime.NumCPU())
	bm.ResetTimer()
	bm.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lim.Pace(done)
		}
	})
}

func TestBucketLimiterLowRates(t *testing.T) {
	expectations := []struct {
		rate     uint64
		duration time.Duration
	}{
		{1, 1 * time.Second},
		{10, 1 * time.Second},
		{15, 1 * time.Second},
		{50, 1 * time.Second},
		{100, 1 * time.Second},
		{150, 1 * time.Second},
		{500, 1 * time.Second},
		{1000, 1 * time.Second},
		{1500, 1 * time.Second},
		{5000, 1 * time.Second},
	}
	var lwg sync.WaitGroup
	lwg.Add(len(expectations))
	for i := range expectations {
		exp := expectations[i]
		go func() {
			defer lwg.Done()
			lim := NewBucketLimiter(exp.rate)
			done := make(chan struct{})
			counter := uint64(0)
			waitChan := make(chan struct{})
			go func() {
				defer func() {
					waitChan <- struct{}{}
				}()
				for lim.Pace(done) == cont {
					counter++
				}
			}()
			time.Sleep(exp.duration)
			close(done)
			select {
			case <-waitChan:
				break
			case <-time.After(exp.duration + 100*time.Millisecond):
				t.Error("failed to complete: ", exp)
				return
			}
			expcounter := float64(exp.rate) * exp.duration.Seconds()
			if float64(counter) < (expcounter*0.9) ||
				float64(counter) > (expcounter*1.1+5) {
				t.Error(expcounter, counter)
			}
		}()
	}
	lwg.Wait()
}

func TestBucketLimiterHighRates(t *testing.T) {
	expectations := []struct {
		rate     uint64
		duration time.Duration
	}{
		{100000, 100 * time.Millisecond},
		{150000, 100 * time.Millisecond},
		{200000, 100 * time.Millisecond},
		{500000, 100 * time.Millisecond},
		{1000000, 100 * time.Millisecond},
	}
	for i := range expectations {
		exp := expectations[i]
		lim := NewBucketLimiter(exp.rate)
		counter := uint64(0)
		done := make(chan struct{})
		waitChan := make(chan struct{})
		go func() {
			defer func() {
				waitChan <- struct{}{}
			}()
			for lim.Pace(done) == cont {
				counter++
			}
		}()
		time.Sleep(exp.duration)
		close(done)
		select {
		case <-waitChan:
		case <-time.After(exp.duration + 50*time.Millisecond):
			t.Error("failed to complete: ", exp)
			return
		}
		expcounter := float64(exp.rate) * exp.duration.Seconds()
		if float64(counter) < (expcounter*0.9) ||
			float64(counter) > (expcounter*1.1) {
			t.Error(expcounter, counter)
		}
	}
}

func BenchmarkBucketLimiter(bm *testing.B) {
	lim := NewBucketLimiter(maxRps)
	done := make(chan struct{})
	bm.SetParallelism(int(defaultNumberOfConns) / runtime.NumCPU())
	bm.ResetTimer()
	bm.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			lim.Pace(done)
		}
	})
}
