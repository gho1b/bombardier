package bombardier

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/gho1b/bombardier/internal"

	"github.com/cheggaaa/pb"
	fhist "github.com/codesenberg/concurrent/float64/histogram"
	uhist "github.com/codesenberg/concurrent/uint64/histogram"
	uuid "github.com/satori/go.uuid"
)

type Bombardier struct {
	bytesRead, bytesWritten int64

	// HTTP codes
	req1xx uint64
	req2xx uint64
	req3xx uint64
	req4xx uint64
	req5xx uint64
	others uint64

	conf        Config
	barrier     CompletionBarrier
	ratelimiter Limiter
	wg          sync.WaitGroup

	timeTaken time.Duration
	latencies *uhist.Histogram
	requests  *fhist.Histogram

	client   Client
	doneChan chan struct{}

	// RPS metrics
	rpl   sync.Mutex
	reqs  int64
	start time.Time

	// Errors
	errors *ErrorMap

	// Progress bar
	bar *pb.ProgressBar

	// Output
	out      io.Writer
	template *template.Template
}

func NewBombardier(c Config) (*Bombardier, error) {
	if err := c.CheckArgs(); err != nil {
		return nil, err
	}
	b := new(Bombardier)
	b.conf = c
	b.latencies = uhist.Default()
	b.requests = fhist.Default()

	if b.conf.TestType() == counted {
		b.bar = pb.New64(int64(*b.conf.numReqs))
		b.bar.ShowSpeed = true
	} else if b.conf.TestType() == timed {
		b.bar = pb.New64(b.conf.duration.Nanoseconds() / 1e9)
		b.bar.ShowCounters = false
		b.bar.ShowPercent = false
	}
	b.bar.ManualUpdate = true

	if b.conf.TestType() == counted {
		b.barrier = NewCountingCompletionBarrier(*b.conf.numReqs)
	} else {
		b.barrier = NewTimedCompletionBarrier(*b.conf.duration)
	}

	if b.conf.rate != nil {
		b.ratelimiter = NewBucketLimiter(*b.conf.rate)
	} else {
		b.ratelimiter = &Nooplimiter{}
	}

	b.out = os.Stdout

	tlsConfig, err := GenerateTLSConfig(c)
	if err != nil {
		return nil, err
	}

	var (
		pbody *string
		bsp   BodyStreamProducer
	)
	if c.stream {
		if c.bodyFilePath != "" {
			bsp = func() (io.ReadCloser, error) {
				return os.Open(c.bodyFilePath)
			}
		} else {
			bsp = func() (io.ReadCloser, error) {
				return ioutil.NopCloser(
					ProxyReader{strings.NewReader(c.body)},
				), nil
			}
		}
	} else {
		pbody = &c.body
		if c.bodyFilePath != "" {
			var bodyBytes []byte
			bodyBytes, err = ioutil.ReadFile(c.bodyFilePath)
			if err != nil {
				return nil, err
			}
			sbody := string(bodyBytes)
			pbody = &sbody
		}
	}

	cc := &ClientOpts{
		HTTP2:             false,
		maxConns:          c.numConns,
		timeout:           c.timeout,
		tlsConfig:         tlsConfig,
		disableKeepAlives: c.disableKeepAlives,

		headers:      c.headers,
		url:          c.url,
		method:       c.method,
		body:         pbody,
		bodProd:      bsp,
		bytesRead:    &b.bytesRead,
		bytesWritten: &b.bytesWritten,
	}
	b.client = MakeHTTPClient(c.clientType, cc)

	if !b.conf.printProgress {
		b.bar.Output = ioutil.Discard
		b.bar.NotPrint = true
	}

	b.template, err = b.PrepareTemplate()
	if err != nil {
		return nil, err
	}

	b.wg.Add(int(c.numConns))
	b.errors = NewErrorMap()
	b.doneChan = make(chan struct{}, 2)
	return b, nil
}

func MakeHTTPClient(clientType ClientTyp, cc *ClientOpts) Client {
	var cl Client
	switch clientType {
	case nhttp1:
		cl = NewHTTPClient(cc)
	case nhttp2:
		cc.HTTP2 = true
		cl = NewHTTPClient(cc)
	case fhttp:
		fallthrough
	default:
		cl = NewFastHTTPClient(cc)
	}
	return cl
}

func (b *Bombardier) PrepareTemplate() (*template.Template, error) {
	var (
		templateBytes []byte
		err           error
	)
	switch f := b.conf.format.(type) {
	case KnownFormat:
		templateBytes = f.Template()
	case UserDefinedTemplate:
		templateBytes, err = ioutil.ReadFile(string(f))
		if err != nil {
			return nil, err
		}
	default:
		panic("Format can't be nil at this point, this is a bug")
	}
	outputTemplate, err := template.New("output-Template").
		Funcs(template.FuncMap{
			"WithLatencies": func() bool {
				return b.conf.printLatencies
			},
			"FormatBinary": FormatBinary,
			"FormatTimeUs": FormatTimeUs,
			"FormatTimeUsUint64": func(us uint64) string {
				return FormatTimeUs(float64(us))
			},
			"FloatsToArray": func(ps ...float64) []float64 {
				return ps
			},
			"Multiply": func(num, coeff float64) float64 {
				return num * coeff
			},
			"StringToBytes": func(s string) []byte {
				return []byte(s)
			},
			"UUIDV1": uuid.NewV1,
			"UUIDV2": uuid.NewV2,
			"UUIDV3": uuid.NewV3,
			"UUIDV4": uuid.NewV4,
			"UUIDV5": uuid.NewV5,
		}).Parse(string(templateBytes))

	if err != nil {
		return nil, err
	}
	return outputTemplate, nil
}

func (b *Bombardier) WriteStatistics(
	code int, usTaken uint64,
) {
	b.latencies.Increment(usTaken)
	b.rpl.Lock()
	b.reqs++
	b.rpl.Unlock()
	var counter *uint64
	switch code / 100 {
	case 1:
		counter = &b.req1xx
	case 2:
		counter = &b.req2xx
	case 3:
		counter = &b.req3xx
	case 4:
		counter = &b.req4xx
	case 5:
		counter = &b.req5xx
	default:
		counter = &b.others
	}
	atomic.AddUint64(counter, 1)
}

func (b *Bombardier) PerformSingleRequest() {
	code, usTaken, err := b.client.Do()
	if err != nil {
		b.errors.Add(err)
	}
	b.WriteStatistics(code, usTaken)
}

func (b *Bombardier) Worker() {
	done := b.barrier.Done()
	for b.barrier.TryGrabWork() {
		if b.ratelimiter.Pace(done) == brk {
			break
		}
		b.PerformSingleRequest()
		b.barrier.JobDone()
	}
}

func (b *Bombardier) BarUpdater() {
	done := b.barrier.Done()
	for {
		select {
		case <-done:
			b.bar.Set64(b.bar.Total)
			b.bar.Update()
			b.bar.Finish()
			if b.conf.printProgress {
				fmt.Fprintln(b.out, "Done!")
			}
			b.doneChan <- struct{}{}
			return
		default:
			current := int64(b.barrier.Completed() * float64(b.bar.Total))
			b.bar.Set64(current)
			b.bar.Update()
			time.Sleep(b.bar.RefreshRate)
		}
	}
}

func (b *Bombardier) RateMeter() {
	requestsInterval := 10 * time.Millisecond
	if b.conf.rate != nil {
		requestsInterval, _ = Estimate(*b.conf.rate, rateLimitInterval)
	}
	requestsInterval += 10 * time.Millisecond
	ticker := time.NewTicker(requestsInterval)
	defer ticker.Stop()
	done := b.barrier.Done()
	for {
		select {
		case <-ticker.C:
			b.RecordRps()
			continue
		case <-done:
			b.wg.Wait()
			b.RecordRps()
			b.doneChan <- struct{}{}
			return
		}
	}
}

func (b *Bombardier) RecordRps() {
	b.rpl.Lock()
	duration := time.Since(b.start)
	reqs := b.reqs
	b.reqs = 0
	b.start = time.Now()
	b.rpl.Unlock()

	reqsf := float64(reqs) / duration.Seconds()
	b.requests.Increment(reqsf)
}

func (b *Bombardier) Bombard() {
	if b.conf.printIntro {
		b.PrintIntro()
	}
	b.bar.Start()
	bombardmentBegin := time.Now()
	b.start = time.Now()
	for i := uint64(0); i < b.conf.numConns; i++ {
		go func() {
			defer b.wg.Done()
			b.Worker()
		}()
	}
	go b.RateMeter()
	go b.BarUpdater()
	b.wg.Wait()
	b.timeTaken = time.Since(bombardmentBegin)
	<-b.doneChan
	<-b.doneChan
}

func (b *Bombardier) PrintIntro() {
	if b.conf.TestType() == counted {
		fmt.Fprintf(b.out,
			"Bombarding %v with %v request(s) using %v connection(s)\n",
			b.conf.url, *b.conf.numReqs, b.conf.numConns)
	} else if b.conf.TestType() == timed {
		fmt.Fprintf(b.out, "Bombarding %v for %v using %v connection(s)\n",
			b.conf.url, *b.conf.duration, b.conf.numConns)
	}
}

func (b *Bombardier) GatherInfo() internal.TestInfo {
	info := internal.TestInfo{
		Spec: internal.Spec{
			NumberOfConnections: b.conf.numConns,

			Method: b.conf.method,
			URL:    b.conf.url,

			Body:         b.conf.body,
			BodyFilePath: b.conf.bodyFilePath,

			CertPath: b.conf.certPath,
			KeyPath:  b.conf.keyPath,

			Stream:     b.conf.stream,
			Timeout:    b.conf.timeout,
			ClientType: internal.ClientType(b.conf.clientType),

			Rate: b.conf.rate,
		},
		Result: internal.Results{
			BytesRead:    b.bytesRead,
			BytesWritten: b.bytesWritten,
			TimeTaken:    b.timeTaken,

			Req1XX: b.req1xx,
			Req2XX: b.req2xx,
			Req3XX: b.req3xx,
			Req4XX: b.req4xx,
			Req5XX: b.req5xx,
			Others: b.others,

			Latencies: b.latencies,
			Requests:  b.requests,
		},
	}

	testType := b.conf.TestType()
	info.Spec.TestType = internal.TestType(testType)
	if testType == timed {
		info.Spec.TestDuration = *b.conf.duration
	} else if testType == counted {
		info.Spec.NumberOfRequests = *b.conf.numReqs
	}

	if b.conf.headers != nil {
		for _, h := range *b.conf.headers {
			info.Spec.Headers = append(info.Spec.Headers,
				internal.Header{
					Key:   h.key,
					Value: h.value,
				})
		}
	}

	for _, ewc := range b.errors.ByFrequency() {
		info.Result.Errors = append(info.Result.Errors,
			internal.ErrorWithCount{
				Error: ewc.error,
				Count: ewc.count,
			})
	}

	return info
}

func (b *Bombardier) PrintStats() {
	info := b.GatherInfo()
	err := b.template.Execute(b.out, info)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (b *Bombardier) RedirectOutputTo(out io.Writer) {
	b.bar.Output = out
	b.out = out
}

func (b *Bombardier) DisableOutput() {
	b.RedirectOutputTo(ioutil.Discard)
	b.bar.NotPrint = true
}

func main() {
	cfg, err := parser.Parse(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(exitFailure)
	}
	bombardier, err := NewBombardier(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(exitFailure)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		bombardier.barrier.Cancel()
	}()
	bombardier.Bombard()
	if bombardier.conf.printResult {
		bombardier.PrintStats()
	}
}
