package main

import (
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The page's sockets, carried over the Wails bus.
//
// A desktop window loads the page from wails://, where a WebSocket has nowhere
// to go: the webview asks a scheme handler for every request and that handler
// cannot carry an upgrade. So the page asks Go to open the socket instead —
// Go dials it on the loopback door like any other process — and the two ends
// meet on the bus: what the socket says is an event named for the tunnel, what
// the page says is a binding call. The protocol inside is untouched, so the
// board's /ws works against this machine's board and an attached server's
// alike, and so does /acp/events/ws.

// tunnelPaths are the sockets a page may ask for.
var tunnelPaths = map[string]bool{"/ws": true, "/acp/events/ws": true}

// tunnelMessage is what the page hears on "socket:<id>": a frame, or the end.
type tunnelMessage struct {
	Data   string `json:"data,omitempty"`
	Closed bool   `json:"closed,omitempty"`
	Code   int    `json:"code,omitempty"`
}

// tunnel is one socket and the lock its writes take: gorilla allows one writer
// at a time, and the page's calls can overlap.
type tunnel struct {
	conn  *websocket.Conn
	write sync.Mutex
}

type socketTunnels struct {
	mu    sync.Mutex
	conns map[string]*tunnel
	// base is the loopback door, "127.0.0.1:port".
	base func() string
	emit func(name string, data any)
}

func newSocketTunnels(base func() string, wapp func() *application.App) *socketTunnels {
	return &socketTunnels{
		conns: map[string]*tunnel{},
		base:  base,
		emit: func(name string, data any) {
			// Straight onto the bus, not through the emitter: the emitter
			// also feeds /acp/events/ws, and a tunnel of that socket would
			// hear its own frames come back.
			if app := wapp(); app != nil {
				app.Event.Emit(name, data)
			}
		},
	}
}

// open dials path under the id the page chose. The page subscribes to the
// event before it asks, so nothing the socket says first can be missed.
func (t *socketTunnels) open(id, path string) error {
	if id == "" || !tunnelPaths[path] {
		return errors.New("no such socket")
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+t.base()+path, http.Header{})
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

// closeAll ends every tunnel; a page that reloads opens new ones.
func (t *socketTunnels) closeAll() {
	t.mu.Lock()
	conns := t.conns
	t.conns = map[string]*tunnel{}
	t.mu.Unlock()
	for _, tun := range conns {
		_ = tun.conn.Close()
	}
	if len(conns) > 0 {
		log.Printf("sockets: closed %d tunnels", len(conns))
	}
}
