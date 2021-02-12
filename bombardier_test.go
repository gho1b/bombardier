package main

import (
	"bytes"
	"container/ring"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestBombardierShouldFireSpecifiedNumberOfRequests(t *testing.T) {
	testAllClients(t, testBombardierShouldFireSpecifiedNumberOfRequests)
}

func testBombardierShouldFireSpecifiedNumberOfRequests(
	clientType ClientTyp, t *testing.T,
) {
	reqsReceived := uint64(0)
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&reqsReceived, 1)
		}),
	)
	defer s.Close()
	numReqs := uint64(100)
	noHeaders := new(HeadersList)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		url:        s.URL,
		headers:    noHeaders,
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
	if reqsReceived != numReqs {
		t.Fail()
	}
}

func TestBombardierShouldFinish(t *testing.T) {
	testAllClients(t, testBombardierShouldFinish)
}

func testBombardierShouldFinish(clientType ClientTyp, t *testing.T) {
	reqsReceived := uint64(0)
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			atomic.AddUint64(&reqsReceived, 1)
		}),
	)
	defer s.Close()
	noHeaders := new(HeadersList)
	desiredTestDuration := 1 * time.Second
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		duration:   &desiredTestDuration,
		url:        s.URL,
		headers:    noHeaders,
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	waitCh := make(chan struct{})
	go func() {
		b.Bombard()
		waitCh <- struct{}{}
	}()
	select {
	case <-waitCh:
	// Do nothing here
	case <-time.After(desiredTestDuration + 5*time.Second):
		t.Fail()
	}
	if reqsReceived == 0 {
		t.Fail()
	}
}

func TestBombardierShouldSendHeaders(t *testing.T) {
	testAllClients(t, testBombardierShouldSendHeaders)
}

func testBombardierShouldSendHeaders(clientType ClientTyp, t *testing.T) {
	requestHeaders := HeadersList([]Header{
		{"Header1", "Value1"},
		{"Header-Two", "value-two"},
	})

	// It's a bit hacky, but FastHTTP can't send Host Header correctly
	// as of now
	if clientType != fhttp {
		requestHeaders = append(requestHeaders, Header{"Host", "web"})
	}

	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			for _, h := range requestHeaders {
				av := r.Header.Get(h.key)
				if h.key == "Host" {
					av = r.Host
				}
				if av != h.value {
					t.Logf("%q <-> %q", av, h.value)
					t.Fail()
				}
			}
		}),
	)
	defer s.Close()
	numReqs := uint64(1)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		url:        s.URL,
		headers:    &requestHeaders,
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
}

func TestBombardierHTTPCodeRecording(t *testing.T) {
	testAllClients(t, testBombardierHTTPCodeRecording)
}

func testBombardierHTTPCodeRecording(clientType ClientTyp, t *testing.T) {
	cs := []int{200, 302, 404, 505, 606, 707}
	codes := ring.New(len(cs))
	for _, v := range cs {
		codes.Value = v
		codes = codes.Next()
	}
	codes = codes.Next()
	var m sync.Mutex
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			m.Lock()
			nextCode := codes.Value.(int)
			codes = codes.Next()
			m.Unlock()
			if nextCode/100 == 3 {
				rw.Header().Set("Location", "http://localhost:666")
			}
			rw.WriteHeader(nextCode)
		}),
	)
	defer s.Close()
	eachCodeCount := uint64(10)
	numReqs := uint64(len(cs)) * eachCodeCount
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
	expectation := []struct {
		name     string
		reqsGot  uint64
		expected uint64
	}{
		{"errored", b.others, eachCodeCount * 2},
		{"2xx", b.req2xx, eachCodeCount},
		{"3xx", b.req3xx, eachCodeCount},
		{"4xx", b.req4xx, eachCodeCount},
		{"5xx", b.req5xx, eachCodeCount},
	}
	for _, e := range expectation {
		if e.reqsGot != e.expected {
			t.Error(e.name, e.reqsGot, e.expected)
		}
	}
	t.Logf("%+v", b.errors.ByFrequency())
}

func TestBombardierTimeoutRecoding(t *testing.T) {
	testAllClients(t, testBombardierTimeoutRecoding)
}

func testBombardierTimeoutRecoding(clientType ClientTyp, t *testing.T) {
	shortTimeout := 10 * time.Millisecond
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			time.Sleep(shortTimeout * 2)
		}),
	)
	defer s.Close()
	numReqs := uint64(10)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		duration:   nil,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    shortTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
	if b.errors.Sum() != numReqs {
		t.Fail()
	}
}

func TestBombardierThroughputRecording(t *testing.T) {
	testAllClients(t, testBombardierThroughputRecording)
}

func testBombardierThroughputRecording(clientType ClientTyp, t *testing.T) {
	responseSize := 1024
	response := bytes.Repeat([]byte{'a'}, responseSize)
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, err := rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	numReqs := uint64(10)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
	if b.bytesRead == 0 || b.bytesWritten == 0 {
		t.Error(b.bytesRead, b.bytesWritten)
	}
}

func TestBombardierStatsPrinting(t *testing.T) {
	responseSize := 1024
	response := bytes.Repeat([]byte{'a'}, responseSize)
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, err := rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	numReqs := uint64(10)
	b, e := NewBombardier(Config{
		numConns:       defaultNumberOfConns,
		numReqs:        &numReqs,
		url:            s.URL,
		headers:        new(HeadersList),
		timeout:        defaultTimeout,
		method:         "GET",
		body:           "",
		printLatencies: true,
		printIntro:     true,
		printProgress:  true,
		printResult:    true,
		format:         KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	dummy := errors.New("dummy error")
	b.errors.Add(dummy)

	out := new(bytes.Buffer)
	b.RedirectOutputTo(out)
	b.Bombard()

	b.PrintStats()
	l := out.Len()
	// Here we only test if anything is written
	if l == 0 {
		t.Fail()
	}
}

func TestBombardierErrorIfFailToReadClientCert(t *testing.T) {
	numReqs := uint64(10)
	_, e := NewBombardier(Config{
		numConns:       defaultNumberOfConns,
		numReqs:        &numReqs,
		url:            "http://localhost",
		headers:        new(HeadersList),
		timeout:        defaultTimeout,
		method:         "GET",
		body:           "",
		printLatencies: true,
		certPath:       "certPath",
		keyPath:        "keyPath",
		format:         KnownFormat("plain-text"),
	})
	if e == nil {
		t.Fail()
	}
}

func TestBombardierClientCerts(t *testing.T) {
	testAllClients(t, testBombardierClientCerts)
}

func testBombardierClientCerts(clientType ClientTyp, t *testing.T) {
	clientCert, err := tls.LoadX509KeyPair("testclient.cert", "testclient.key")
	if err != nil {
		t.Error(err)
		return
	}

	x509Cert, err := x509.ParseCertificate(clientCert.Certificate[0])
	if err != nil {
		t.Error(err)
		return
	}

	server := fasthttp.Server{
		DisableKeepalive: true,
		Handler: func(ctx *fasthttp.RequestCtx) {
			certs := ctx.TLSConnectionState().PeerCertificates
			if numCerts := len(certs); numCerts != 1 {
				t.Errorf("expected 1 cert, but got %v", numCerts)
				ctx.Error("invalid number of certs", http.StatusBadRequest)
				return
			}

			cert := certs[0]
			if !cert.Equal(x509Cert) {
				t.Error("certificates don't match")
				ctx.Error("wrong cert", http.StatusBadRequest)
				return
			}

			ctx.Success("text/plain; charset=utf-8", []byte("OK"))
		},
	}

	ln, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		t.Error(err)
		return
	}

	go func() {
		serr := server.ServeTLS(ln, "testserver.cert", "testserver.key")
		if serr != nil {
			t.Error(err)
		}
	}()

	numReqs := uint64(1)
	b, e := NewBombardier(Config{
		numConns:       defaultNumberOfConns,
		numReqs:        &numReqs,
		url:            "https://localhost:8080/",
		headers:        new(HeadersList),
		timeout:        defaultTimeout,
		method:         "GET",
		body:           "",
		printLatencies: true,
		certPath:       "testclient.cert",
		keyPath:        "testclient.key",
		insecure:       true,
		clientType:     clientType,
		format:         KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()

	b.Bombard()
	if b.req2xx != 1 {
		t.Error("no requests succeeded")
	}

	err = ln.Close()
	if err != nil {
		t.Error(err)
	}
	// TODO(codesenberg): remove. Another hacky attempt to fix Travis CI's
	// slowness
	time.Sleep(100 * time.Millisecond)
}

func TestBombardierRateLimiting(t *testing.T) {
	testAllClients(t, testBombardierRateLimiting)
}

func testBombardierRateLimiting(clientType ClientTyp, t *testing.T) {
	responseSize := 1024
	response := bytes.Repeat([]byte{'a'}, responseSize)
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			_, err := rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	rate := uint64(5000)
	testDuration := 1 * time.Second
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		duration:   &testDuration,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		rate:       &rate,
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()
	b.Bombard()
	if float64(b.req2xx) < float64(rate)*0.75 ||
		float64(b.req2xx) > float64(rate)*1.25 {
		t.Error(rate, b.req2xx)
	}
}

func testAllClients(parent *testing.T, testFun func(ClientTyp, *testing.T)) {
	clients := []ClientTyp{fhttp, nhttp1, nhttp2}
	for _, ct := range clients {
		parent.Run(ct.String(), func(t *testing.T) {
			testFun(ct, t)
		})
	}
}

func TestBombardierSendsBody(t *testing.T) {
	testAllClients(t, testBombardierSendsBody)
}

func testBombardierSendsBody(clientType ClientTyp, t *testing.T) {
	response := []byte("OK")
	requestBody := "abracadabra"
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(body) != requestBody {
				t.Errorf("Expected %v, but got %v", requestBody, string(body))
			}
			_, err = rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	one := uint64(1)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &one,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    defaultTimeout,
		method:     "POST",
		body:       requestBody,
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()
	b.Bombard()
}

func TestBombardierSendsBodyFromFile(t *testing.T) {
	testAllClients(t, testBombardierSendsBodyFromFile)
}

func testBombardierSendsBodyFromFile(clientType ClientTyp, t *testing.T) {
	response := []byte("OK")
	bodyPath := "testbody.txt"
	requestBody, err := ioutil.ReadFile(bodyPath)
	if err != nil {
		t.Error(err)
		return
	}
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(body) != string(requestBody) {
				t.Errorf("Expected %v, but got %v", string(requestBody), string(body))
			}
			_, err = rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	one := uint64(1)
	b, e := NewBombardier(Config{
		numConns:     defaultNumberOfConns,
		numReqs:      &one,
		url:          s.URL,
		headers:      new(HeadersList),
		timeout:      defaultTimeout,
		method:       "POST",
		bodyFilePath: bodyPath,
		clientType:   clientType,
		format:       KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()
	b.Bombard()
}

func TestBombardierFileDoesntExist(t *testing.T) {
	bodyPath := "/does/not/exist.forreal"
	_, e := NewBombardier(Config{
		numConns:     defaultNumberOfConns,
		url:          "http://example.com",
		headers:      new(HeadersList),
		timeout:      defaultTimeout,
		method:       "POST",
		bodyFilePath: bodyPath,
		format:       KnownFormat("plain-text"),
	})
	_, ok := e.(*os.PathError)
	if !ok {
		t.Errorf("Expected to Get PathError, but got %v", e)
	}
}

func TestBombardierStreamsBody(t *testing.T) {
	testAllClients(t, testBombardierStreamsBody)
}

func testBombardierStreamsBody(clientType ClientTyp, t *testing.T) {
	response := []byte("OK")
	requestBody := "abracadabra"
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if te := r.TransferEncoding; !reflect.DeepEqual(te, []string{"chunked"}) {
				t.Errorf("Expected chunked transfer encoding, but got %v", te)
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(body) != requestBody {
				t.Errorf("Expected %v, but got %v", requestBody, string(body))
			}
			_, err = rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	one := uint64(1)
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &one,
		url:        s.URL,
		headers:    new(HeadersList),
		timeout:    defaultTimeout,
		method:     "POST",
		body:       requestBody,
		stream:     true,
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()
	b.Bombard()
}

func TestBombardierStreamsBodyFromFile(t *testing.T) {
	testAllClients(t, testBombardierStreamsBodyFromFile)
}

func testBombardierStreamsBodyFromFile(clientType ClientTyp, t *testing.T) {
	response := []byte("OK")
	bodyPath := "testbody.txt"
	requestBody, err := ioutil.ReadFile(bodyPath)
	if err != nil {
		t.Error(err)
		return
	}
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if te := r.TransferEncoding; !reflect.DeepEqual(te, []string{"chunked"}) {
				t.Errorf("Expected chunked transfer encoding, but got %v", te)
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if string(body) != string(requestBody) {
				t.Errorf("Expected %v, but got %v", string(requestBody), string(body))
			}
			_, err = rw.Write(response)
			if err != nil {
				t.Error(err)
			}
		}),
	)
	defer s.Close()
	one := uint64(1)
	b, e := NewBombardier(Config{
		numConns:     defaultNumberOfConns,
		numReqs:      &one,
		url:          s.URL,
		headers:      new(HeadersList),
		timeout:      defaultTimeout,
		method:       "POST",
		bodyFilePath: bodyPath,
		stream:       true,
		clientType:   clientType,
		format:       KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
		return
	}
	b.DisableOutput()
	b.Bombard()
}

func TestBombardierShouldSendCustomHostHeader(t *testing.T) {
	testAllClients(t, testBombardierShouldSendCustomHostHeader)
}

func testBombardierShouldSendCustomHostHeader(
	clientType ClientTyp, t *testing.T,
) {
	host := "custom-host"
	s := httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.Host != host {
				t.Errorf("Host must be %q, but it's %q", host, r.Host)
			}
		}),
	)
	defer s.Close()
	numReqs := uint64(100)
	headers := HeadersList([]Header{
		{"Host", host},
	})
	b, e := NewBombardier(Config{
		numConns:   defaultNumberOfConns,
		numReqs:    &numReqs,
		url:        s.URL,
		headers:    &headers,
		timeout:    defaultTimeout,
		method:     "GET",
		body:       "",
		clientType: clientType,
		format:     KnownFormat("plain-text"),
	})
	if e != nil {
		t.Error(e)
	}
	b.DisableOutput()
	b.Bombard()
}
