package main

import "io"

type ProxyReader struct {
	io.Reader
}
