package main

import (
	"errors"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// socketTunnels hold the board's WebSocket for a desktop page, which is on
// wails:// and cannot open one: frames arrive on the bus as "socket:<id>",
// and go out through SendSocket.

var tunnelPaths = map[string]bool{"/ws": true}

type tunnelMessage struct {
	Data   string `json:"data,omitempty"`
	Closed bool   `json:"closed,omitempty"`
	Code   int    `json:"code,omitempty"`
}

type tunnel struct {
	conn  *websocket.Conn
	write sync.Mutex
}

type socketTunnels struct {
	mu    sync.Mutex
	conns map[string]*tunnel
	dial  func(path string) (*websocket.Conn, error)
	emit  func(name string, data any)
}

func newSocketTunnels(dial func(string) (*websocket.Conn, error), wapp func() *application.App) *socketTunnels {
	return &socketTunnels{
		conns: map[string]*tunnel{},
		dial:  dial,
		emit: func(name string, data any) {
			if app := wapp(); app != nil {
				app.Event.Emit(name, data)
			}
		},
	}
}

func (t *socketTunnels) open(id, path string) error {
	if id == "" || !tunnelPaths[path] {
		return errors.New("no such socket")
	}
	conn, err := t.dial(path)
	if err != nil {
		return err
	}
	t.mu.Lock()
	if old := t.conns[id]; old != nil {
		_ = old.conn.Close()
	}
	t.conns[id] = &tunnel{conn: conn}
	t.mu.Unlock()

	go func() {
		event := "socket:" + id
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				code := websocket.CloseAbnormalClosure
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					code = closeErr.Code
				}
				t.drop(id, conn)
				t.emit(event, tunnelMessage{Closed: true, Code: code})
				return
			}
			t.emit(event, tunnelMessage{Data: string(data)})
		}
	}()
	return nil
}

func (t *socketTunnels) send(id, data string) error {
	t.mu.Lock()
	tun := t.conns[id]
	t.mu.Unlock()
	if tun == nil {
		return errors.New("socket closed")
	}
	tun.write.Lock()
	defer tun.write.Unlock()
	return tun.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

func (t *socketTunnels) close(id string) {
	t.mu.Lock()
	tun := t.conns[id]
	delete(t.conns, id)
	t.mu.Unlock()
	if tun != nil {
		_ = tun.conn.Close()
	}
}

func (t *socketTunnels) drop(id string, conn *websocket.Conn) {
	t.mu.Lock()
	if tun := t.conns[id]; tun != nil && tun.conn == conn {
		delete(t.conns, id)
	}
	t.mu.Unlock()
	_ = conn.Close()
}
