//go:build ignore

// A low-overhead WebSocket connection holder for connectivity tests.
// It continuously reads each socket so gorilla/websocket can answer server pings.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type fixtureUser struct { Token string `json:"token"` }
type counters struct {
	opened atomic.Int64
	closed atomic.Int64
	handshakeFailures atomic.Int64
	rampReconnects atomic.Int64
	rampConnectionErrors atomic.Int64
	socketErrors atomic.Int64
	earlyClosed atomic.Int64
	serverPings atomic.Int64
	active atomic.Int64
}

func main() {
	usersFile := flag.String("users", "", "users.json fixture")
	host := flag.String("host", "", "WebSocket base URL")
	total := flag.Int("connections", 10000, "connection count")
	ramp := flag.Duration("ramp", 2*time.Minute, "connection ramp")
	hold := flag.Duration("hold", 10*time.Minute, "full-connection hold")
	summaryFile := flag.String("summary", "", "k6-compatible summary output")
	flag.Parse()
	if *usersFile == "" || *host == "" || *summaryFile == "" { log.Fatal("users, host and summary are required") }
	data, err := os.ReadFile(*usersFile); if err != nil { log.Fatal(err) }
	var users []fixtureUser; if err := json.Unmarshal(data, &users); err != nil { log.Fatal(err) }
	if len(users) < *total { log.Fatalf("fixture has %d users, need %d", len(users), *total) }

	start := time.Now().Add(10 * time.Second)
	fullAt := start.Add(*ramp)
	closeAt := fullAt.Add(*hold)
	fmt.Printf("LOAD_T0=%d\n", start.UnixMilli())
	var c counters
	var wg sync.WaitGroup
	for i := 0; i < *total; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Reserve the last 10% of the ramp as a retry/settling window so the
			// final scheduled connection has time to retry before the hold begins.
			scheduleSpan := time.Duration(float64(*ramp) * 0.9)
			target := start.Add(time.Duration(int64(scheduleSpan) * int64(index) / int64(*total)))
			if wait := time.Until(target); wait > 0 { time.Sleep(wait) }
			attempt := 0
			var conn *websocket.Conn
			for time.Now().Before(fullAt) {
				u, _ := url.Parse(*host + "/api/v1/ws")
				q := u.Query(); q.Set("token", users[index].Token); u.RawQuery = q.Encode()
				d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
				conn, _, err = d.Dial(u.String(), nil)
				if err == nil { break }
				c.rampConnectionErrors.Add(1); c.rampReconnects.Add(1); attempt++
				backoff := 100 * time.Millisecond * time.Duration(1<<min(attempt-1, 6)); if backoff > 5*time.Second { backoff = 5*time.Second }
				backoff = time.Duration(float64(backoff) * (0.8 + rand.Float64()*0.4))
				if time.Now().Add(backoff).After(fullAt) { break }; time.Sleep(backoff)
			}
			if conn == nil { c.handshakeFailures.Add(1); return }
			c.opened.Add(1); c.active.Add(1)
			defer func(){ c.active.Add(-1); c.closed.Add(1); conn.Close() }()
			defaultPing := conn.PingHandler()
			conn.SetPingHandler(func(appData string) error { c.serverPings.Add(1); return defaultPing(appData) })
			_ = conn.SetReadDeadline(closeAt.Add(5 * time.Second))
			for {
				_, _, err = conn.ReadMessage()
				if err != nil {
					if time.Now().Before(closeAt.Add(-100 * time.Millisecond)) { c.socketErrors.Add(1); c.earlyClosed.Add(1) }
					return
				}
			}
		}(i)
	}
	monitorDone := make(chan struct{})
	go func(){ ticker:=time.NewTicker(20*time.Second); defer ticker.Stop(); for { select { case <-ticker.C: fmt.Printf("GO_LOAD elapsed=%ds active=%d opened=%d handshake_failures=%d early_closed=%d pings=%d\n", int(time.Since(start).Seconds()), c.active.Load(), c.opened.Load(), c.handshakeFailures.Load(), c.earlyClosed.Load(), c.serverPings.Load()); case <-monitorDone: return } } }()
	if wait := time.Until(closeAt); wait > 0 { time.Sleep(wait) }
	// Closing connections unblocks every reader and makes the planned close non-error.
	// A short spread avoids an artificial FIN burst.
	deadline := time.Now().Add(5*time.Second)
	for c.active.Load() > 0 && time.Now().Before(deadline) { time.Sleep(50*time.Millisecond) }
	wg.Wait(); close(monitorDone)
	metrics := map[string]any{
		"load_opened": map[string]any{"count":c.opened.Load()}, "load_closed":map[string]any{"count":c.closed.Load()},
		"load_handshake_failures":map[string]any{"count":c.handshakeFailures.Load()}, "load_ramp_reconnects":map[string]any{"count":c.rampReconnects.Load()},
		"load_ramp_connection_errors":map[string]any{"count":c.rampConnectionErrors.Load()}, "load_socket_errors":map[string]any{"count":c.socketErrors.Load()},
		"load_early_closed":map[string]any{"count":c.earlyClosed.Load()}, "load_server_pings":map[string]any{"count":c.serverPings.Load()},
		"load_sent":map[string]any{"count":0}, "load_received":map[string]any{"count":0}, "load_received_frames":map[string]any{"count":0},
		"load_send_slots_skipped":map[string]any{"count":0}, "load_nonmonotonic_delivery":map[string]any{"count":0}, "load_malformed_delivery":map[string]any{"count":0},
	}
	out, _ := json.MarshalIndent(map[string]any{"metrics":metrics}, "", "  ")
	if err := os.WriteFile(*summaryFile, out, 0600); err != nil { log.Fatal(err) }
}
