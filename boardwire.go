package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// boardWire serves the board server's router over in-memory pipes, so the
// board server has no port at all.
type boardWire struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

const boardWireHost = "board.internal"

func newBoardWire() *boardWire {
	return &boardWire{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (w *boardWire) serve(h http.Handler) {
	server := &http.Server{Handler: h, ReadHeaderTimeout: 30 * time.Second}
	go func() { _ = server.Serve(w) }()
}

func (w *boardWire) Accept() (net.Conn, error) {
	select {
	case conn := <-w.conns:
		return conn, nil
	case <-w.done:
		return nil, net.ErrClosed
	}
}

func (w *boardWire) Close() error {
	w.once.Do(func() { close(w.done) })
	return nil
}

func (w *boardWire) Addr() net.Addr { return wireAddr{} }

func (w *boardWire) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case w.conns <- server:
		return client, nil
	case <-w.done:
		_ = client.Close()
		_ = server.Close()
		return nil, net.ErrClosed
	case <-ctx.Done():
		_ = client.Close()
		_ = server.Close()
		return nil, ctx.Err()
	}
}

func (w *boardWire) transport() *http.Transport {
	return &http.Transport{DialContext: w.DialContext, MaxIdleConns: 16, IdleConnTimeout: 90 * time.Second}
}

func (w *boardWire) dialSocket(path string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{NetDialContext: w.DialContext, HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial("ws://"+boardWireHost+path, http.Header{})
	return conn, err
}

type wireAddr struct{}

func (wireAddr) Network() string { return "pipe" }
func (wireAddr) String() string  { return boardWireHost }
