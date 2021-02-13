package bombardier

import (
	"testing"
	"time"
)

var (
	defaultNumberOfReqs = uint64(10000)
)

func TestCanHaveBody(t *testing.T) {
	expectations := []struct {
		in  string
		out bool
	}{
		{"GET", false},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"HEAD", false},
		{"OPTIONS", true},
	}
	for _, e := range expectations {
		if r := CanHaveBody(e.in); r != e.out {
			t.Error(e.in, e.out, r)
		}
	}
}

func TestAllowedHttpMethod(t *testing.T) {
	expectations := []struct {
		in  string
		out bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"TRUNCATE", false},
	}
	for _, e := range expectations {
		if r := AllowedHTTPMethod(e.in); r != e.out {
			t.Logf("Expected f(%v) = %v, but got %v", e.in, e.out, r)
			t.Fail()
		}
	}
}

func TestCheckArgs(t *testing.T) {
	invalidNumberOfReqs := uint64(0)
	smallTestDuration := 99 * time.Millisecond
	negativeTimeoutDuration := -1 * time.Second
	noHeaders := new(HeadersList)
	zeroRate := uint64(0)
	expectations := []struct {
		in  Config
		out error
	}{
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "ftp://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			errInvalidURL,
		},
		{
			Config{
				numConns: 0,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			errInvalidNumberOfConns,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &invalidNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			errInvalidNumberOfRequests,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  nil,
				duration: &smallTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			errInvalidTestDuration,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  negativeTimeoutDuration,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			errNegativeTimeout,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "BODY",
				format:   KnownFormat("plain-text"),
			},
			errBodyNotAllowed,
		},
		{
			Config{
				numConns:     defaultNumberOfConns,
				numReqs:      &defaultNumberOfReqs,
				duration:     &defaultTestDuration,
				url:          "http://localhost:8080",
				headers:      noHeaders,
				timeout:      defaultTimeout,
				method:       "GET",
				bodyFilePath: "testbody.txt",
				format:       KnownFormat("plain-text"),
			},
			errBodyNotAllowed,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				format:   KnownFormat("plain-text"),
			},
			nil,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				certPath: "test_cert.pem",
				keyPath:  "",
				format:   KnownFormat("plain-text"),
			},
			errNoPathToKey,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				body:     "",
				certPath: "",
				keyPath:  "test_key.pem",
				format:   KnownFormat("plain-text"),
			},
			errNoPathToCert,
		},
		{
			Config{
				numConns: defaultNumberOfConns,
				numReqs:  &defaultNumberOfReqs,
				duration: &defaultTestDuration,
				url:      "http://localhost:8080",
				headers:  noHeaders,
				timeout:  defaultTimeout,
				method:   "GET",
				rate:     &zeroRate,
				format:   KnownFormat("plain-text"),
			},
			errZeroRate,
		},
		{
			Config{
				numConns:     defaultNumberOfConns,
				numReqs:      &defaultNumberOfReqs,
				duration:     &defaultTestDuration,
				url:          "http://localhost:8080",
				headers:      noHeaders,
				timeout:      defaultTimeout,
				method:       "POST",
				body:         "abracadabra",
				bodyFilePath: "testbody.txt",
				format:       KnownFormat("plain-text"),
			},
			errBodyProvidedTwice,
		},
	}
	for _, e := range expectations {
		if r := e.in.CheckArgs(); r != e.out {
			t.Logf("Expected (%v).CheckArgs to return %v, but got %v", e.in, e.out, r)
			t.Fail()
		}
		if _, r := NewBombardier(e.in); r != e.out {
			t.Logf("Expected newBombardier(%v) to return %v, but got %v", e.in, e.out, r)
			t.Fail()
		}
	}
}

func TestCheckArgsGarbageUrl(t *testing.T) {
	c := Config{
		numConns: defaultNumberOfConns,
		numReqs:  &defaultNumberOfReqs,
		duration: &defaultTestDuration,
		url:      "8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "GET",
		body:     "",
	}
	if c.CheckArgs() == nil {
		t.Fail()
	}
}

func TestCheckArgsInvalidRequestMethod(t *testing.T) {
	c := Config{
		numConns: defaultNumberOfConns,
		numReqs:  &defaultNumberOfReqs,
		duration: &defaultTestDuration,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "ABRACADABRA",
		body:     "",
	}
	e := c.CheckArgs()
	if e == nil {
		t.Fail()
	}
	if _, ok := e.(*InvalidHTTPMethodError); !ok {
		t.Fail()
	}
}

func TestCheckArgsTestType(t *testing.T) {
	countedConfig := Config{
		numConns: defaultNumberOfConns,
		numReqs:  &defaultNumberOfReqs,
		duration: nil,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "GET",
		body:     "",
	}
	timedConfig := Config{
		numConns: defaultNumberOfConns,
		numReqs:  nil,
		duration: &defaultTestDuration,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "GET",
		body:     "",
	}
	both := Config{
		numConns: defaultNumberOfConns,
		numReqs:  &defaultNumberOfReqs,
		duration: &defaultTestDuration,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "GET",
		body:     "",
	}
	defaultConfig := Config{
		numConns: defaultNumberOfConns,
		numReqs:  nil,
		duration: nil,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  defaultTimeout,
		method:   "GET",
		body:     "",
	}
	if err := countedConfig.CheckArgs(); err != nil ||
		countedConfig.TestType() != counted {
		t.Fail()
	}
	if err := timedConfig.CheckArgs(); err != nil ||
		timedConfig.TestType() != timed {
		t.Fail()
	}
	if err := both.CheckArgs(); err != nil ||
		both.TestType() != counted {
		t.Fail()
	}
	if err := defaultConfig.CheckArgs(); err != nil ||
		defaultConfig.TestType() != timed ||
		defaultConfig.duration != &defaultTestDuration {
		t.Fail()
	}
}

func TestTimeoutMillis(t *testing.T) {
	defaultConfig := Config{
		numConns: defaultNumberOfConns,
		numReqs:  nil,
		duration: nil,
		url:      "http://localhost:8080",
		headers:  nil,
		timeout:  2 * time.Second,
		method:   "GET",
		body:     "",
	}
	if defaultConfig.TimeoutMillis() != 2000000 {
		t.Fail()
	}
}

func TestInvalidHTTPMethodError(t *testing.T) {
	invalidMethod := "NOSUCHMETHOD"
	want := "Unknown HTTP method: " + invalidMethod
	err := &InvalidHTTPMethodError{invalidMethod}
	if got := err.Error(); got != want {
		t.Error(got, want)
	}
}

func TestClientTypToStringConversion(t *testing.T) {
	expectations := []struct {
		in  ClientTyp
		out string
	}{
		{fhttp, "FastHTTP"},
		{nhttp1, "net/http v1.x"},
		{nhttp2, "net/http v2.0"},
		{42, "unknown Client"},
	}
	for _, exp := range expectations {
		act := exp.in.String()
		if act != exp.out {
			t.Errorf("Expected %v, but got %v", exp.out, act)
		}
	}
}

func clientTypeFromString(s string) ClientTyp {
	switch s {
	case "fasthttp":
		return fhttp
	case "http1":
		return nhttp1
	case "http2":
		return nhttp2
	default:
		return fhttp
	}
}
