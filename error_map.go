package bombardier

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

type ErrorMap struct {
	mu sync.RWMutex
	m  map[string]*uint64
}

func NewErrorMap() *ErrorMap {
	em := new(ErrorMap)
	em.m = make(map[string]*uint64)
	return em
}

func (e *ErrorMap) Add(err error) {
	s := err.Error()
	e.mu.RLock()
	c, ok := e.m[s]
	e.mu.RUnlock()
	if !ok {
		e.mu.Lock()
		c, ok = e.m[s]
		if !ok {
			c = new(uint64)
			e.m[s] = c
		}
		e.mu.Unlock()
	}
	atomic.AddUint64(c, 1)
}

func (e *ErrorMap) Get(err error) uint64 {
	s := err.Error()
	e.mu.RLock()
	defer e.mu.RUnlock()
	c := e.m[s]
	if c == nil {
		return uint64(0)
	}
	return *c
}

func (e *ErrorMap) Sum() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sum := uint64(0)
	for _, v := range e.m {
		sum += *v
	}
	return sum
}

type ErrorWithCount struct {
	error string
	count uint64
}

func (ewc *ErrorWithCount) String() string {
	return "<" + ewc.error + ":" +
		strconv.FormatUint(ewc.count, decBase) + ">"
}

type ErrorsByFrequency []*ErrorWithCount

func (ebf ErrorsByFrequency) Len() int {
	return len(ebf)
}

func (ebf ErrorsByFrequency) Less(i, j int) bool {
	return ebf[i].count > ebf[j].count
}

func (ebf ErrorsByFrequency) Swap(i, j int) {
	ebf[i], ebf[j] = ebf[j], ebf[i]
}

func (e *ErrorMap) ByFrequency() ErrorsByFrequency {
	e.mu.RLock()
	byFreq := make(ErrorsByFrequency, 0, len(e.m))
	for err, count := range e.m {
		byFreq = append(byFreq, &ErrorWithCount{err, *count})
	}
	e.mu.RUnlock()
	sort.Sort(byFreq)
	return byFreq
}
