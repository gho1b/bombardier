package main

import (
	"flag"
	"runtime"
	"testing"
	"time"
)

var (
	serverPort = flag.String("port", "8080", "port to use for benchmarks")
	clientType = flag.String("Client-type", "fasthttp",
		"Client to use in benchmarks")
)

var (
	longDuration = 9001 * time.Hour
	highRate     = uint64(1000000)
)

func BenchmarkBombardierSingleReqPerf(b *testing.B) {
	addr := "localhost:" + *serverPort
	benchmarkFireRequest(Config{
		numConns:       defaultNumberOfConns,
		numReqs:        nil,
		duration:       &longDuration,
		url:            "http://" + addr,
		headers:        new(HeadersList),
		timeout:        defaultTimeout,
		method:         "GET",
		body:           "",
		printLatencies: false,
		clientType:     clientTypeFromString(*clientType),
	}, b)
}

func BenchmarkBombardierRateLimitPerf(b *testing.B) {
	addr := "localhost:" + *serverPort
	benchmarkFireRequest(Config{
		numConns:       defaultNumberOfConns,
		numReqs:        nil,
		duration:       &longDuration,
		url:            "http://" + addr,
		headers:        new(HeadersList),
		timeout:        defaultTimeout,
		method:         "GET",
		body:           "",
		printLatencies: false,
		rate:           &highRate,
		clientType:     clientTypeFromString(*clientType),
	}, b)
}

func benchmarkFireRequest(c Config, bm *testing.B) {
	b, e := NewBombardier(c)
	if e != nil {
		bm.Error(e)
	}
	b.DisableOutput()
	bm.SetParallelism(int(defaultNumberOfConns) / runtime.NumCPU())
	bm.ResetTimer()
	bm.RunParallel(func(pb *testing.PB) {
		done := b.barrier.Done()
		for pb.Next() {
			b.ratelimiter.Pace(done)
			b.PerformSingleRequest()
		}
	})
}
