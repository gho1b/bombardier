package main

import (
	"fmt"
	"net/url"
	"sort"
	"time"
)

type Config struct {
	numConns                       uint64
	numReqs                        *uint64
	disableKeepAlives              bool
	duration                       *time.Duration
	url, method, certPath, keyPath string
	body, bodyFilePath             string
	stream                         bool
	headers                        *HeadersList
	timeout                        time.Duration
	// TODO(codesenberg): printLatencies should probably be
	// re(named&maked) into printPercentiles or even let
	// users provide their own percentiles and not just
	// calculate for [0.5, 0.75, 0.9, 0.99]
	printLatencies, insecure bool
	rate                     *uint64
	clientType               ClientTyp

	printIntro, printProgress, printResult bool

	format Format
}

type TestTyp int

const (
	none TestTyp = iota
	timed
	counted
)

type InvalidHTTPMethodError struct {
	method string
}

func (i *InvalidHTTPMethodError) Error() string {
	return fmt.Sprintf("Unknown HTTP method: %v", i.method)
}

func (c *Config) CheckArgs() error {
	c.CheckOrSetDefaultTestType()

	checks := []func() error{
		c.CheckURL,
		c.CheckRate,
		c.CheckRunParameters,
		c.CheckTimeoutDuration,
		c.CheckHTTPParameters,
		c.CheckCertPaths,
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) CheckOrSetDefaultTestType() {
	if c.TestType() == none {
		c.duration = &defaultTestDuration
	}
}

func (c *Config) TestType() TestTyp {
	typ := none
	if c.numReqs != nil {
		typ = counted
	} else if c.duration != nil {
		typ = timed
	}
	return typ
}

func (c *Config) CheckURL() error {
	url, err := url.Parse(c.url)
	if err != nil {
		return err
	}
	if url.Host == "" || (url.Scheme != "http" && url.Scheme != "https") {
		return errInvalidURL
	}
	c.url = url.String()
	return nil
}

func (c *Config) CheckRate() error {
	if c.rate != nil && *c.rate < 1 {
		return errZeroRate
	}
	return nil
}

func (c *Config) CheckRunParameters() error {
	if c.numConns < uint64(1) {
		return errInvalidNumberOfConns
	}
	if c.TestType() == counted && *c.numReqs < uint64(1) {
		return errInvalidNumberOfRequests
	}
	if c.TestType() == timed && *c.duration < time.Second {
		return errInvalidTestDuration
	}
	return nil
}

func (c *Config) CheckTimeoutDuration() error {
	if c.timeout < 0 {
		return errNegativeTimeout
	}
	return nil
}

func (c *Config) CheckHTTPParameters() error {
	if !AllowedHTTPMethod(c.method) {
		return &InvalidHTTPMethodError{method: c.method}
	}
	if !CanHaveBody(c.method) && (c.body != "" || c.bodyFilePath != "") {
		return errBodyNotAllowed
	}
	if c.body != "" && c.bodyFilePath != "" {
		return errBodyProvidedTwice
	}
	return nil
}

func (c *Config) CheckCertPaths() error {
	if c.certPath != "" && c.keyPath == "" {
		return errNoPathToKey
	} else if c.certPath == "" && c.keyPath != "" {
		return errNoPathToCert
	}
	return nil
}

func (c *Config) TimeoutMillis() uint64 {
	return uint64(c.timeout.Nanoseconds() / 1000)
}

func AllowedHTTPMethod(method string) bool {
	i := sort.SearchStrings(httpMethods, method)
	return i < len(httpMethods) && httpMethods[i] == method
}

func CanHaveBody(method string) bool {
	i := sort.SearchStrings(cantHaveBody, method)
	return !(i < len(cantHaveBody) && cantHaveBody[i] == method)
}

type ClientTyp int

const (
	fhttp ClientTyp = iota
	nhttp1
	nhttp2
)

func (ct ClientTyp) String() string {
	switch ct {
	case fhttp:
		return "FastHTTP"
	case nhttp1:
		return "net/http v1.x"
	case nhttp2:
		return "net/http v2.0"
	}
	return "unknown Client"
}
