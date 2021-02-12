package main

import (
	"context"
	"net"
	"sync/atomic"
)

type CountingConn struct {
	net.Conn
	bytesRead, bytesWritten *int64
}

func (cc *CountingConn) Read(b []byte) (n int, err error) {
	n, err = cc.Conn.Read(b)

	if err == nil {
		atomic.AddInt64(cc.bytesRead, int64(n))
	}

	return
}

func (cc *CountingConn) Write(b []byte) (n int, err error) {
	n, err = cc.Conn.Write(b)

	if err == nil {
		atomic.AddInt64(cc.bytesWritten, int64(n))
	}

	return
}

var FasthttpDialFunc = func(
	bytesRead, bytesWritten *int64,
) func(string) (net.Conn, error) {
	return func(address string) (net.Conn, error) {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return nil, err
		}

		wrappedConn := &CountingConn{
			Conn:         conn,
			bytesRead:    bytesRead,
			bytesWritten: bytesWritten,
		}

		return wrappedConn, nil
	}
}

var HttpDialContextFunc = func(
	bytesRead, bytesWritten *int64,
) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}

		wrappedConn := &CountingConn{
			Conn:         conn,
			bytesRead:    bytesRead,
			bytesWritten: bytesWritten,
		}

		return wrappedConn, nil
	}
}
