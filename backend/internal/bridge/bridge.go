// Package bridge implements the WebSocket-over-TLS transport that carries
// SSH traffic end to end. It is the server side of the same idea apps like
// HTTP Injector use on the client: wrap the raw SSH byte stream inside a
// WebSocket connection (itself inside TLS) so it looks like ordinary HTTPS
// traffic on the wire, then unwrap it and hand the bytes to the real local
// sshd. The bridge does not interpret, inspect, or execute anything inside
// that stream - it is a byte-for-byte relay into 127.0.0.1:<sshd port>,
// nothing more. Every connection must present a valid path/host matching
// this deployment's configured subdomain; there is no open-relay mode.
package bridge

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Bridge struct {
	sshTarget   string // "127.0.0.1:22"
	allowedHost string // configured subdomain; "" disables the check (not recommended once a domain exists)
	upgrader    websocket.Upgrader
}

// New builds a Bridge. allowedHost, when non-empty, is the only Host header
// this bridge will accept (e.g. "ssh.example.com") - it comes from the same
// subdomain the admin configured certificates for. Leaving it empty is only
// appropriate before a domain has been set up; once one exists, every
// deployment should set it, so the bridge cannot be driven by a request
// aimed at some other vhost that happens to resolve to this IP.
func New(sshdPort int, allowedHost string) *Bridge {
	return &Bridge{
		sshTarget:   net.JoinHostPort("127.0.0.1", itoa(sshdPort)),
		allowedHost: normalizeHost(allowedHost),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  32 * 1024,
			WriteBufferSize: 32 * 1024,
			// The point of this bridge is to accept the exact kind of
			// WebSocket handshake HTTP Injector-style clients send, which
			// do not send a browser Origin header - so Origin is not used
			// as an access check here. Access control is the SSH account's
			// own username/password or key, checked by sshd itself on the
			// far end of this relay, exactly as if the client had opened a
			// raw TCP connection to sshd directly.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// normalizeHost strips a port and lowercases a Host header value so
// "Ssh.Example.com:8443" and "ssh.example.com" compare equal.
func normalizeHost(h string) string {
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	return strings.ToLower(strings.TrimSpace(h))
}

// hostAllowed reports whether requestHost may use this bridge. An empty
// configuredHost means the check is disabled (allow everything) - used only
// when no domain has been configured yet.
func hostAllowed(configuredHost, requestHost string) bool {
	if configuredHost == "" {
		return true
	}
	return normalizeHost(requestHost) == configuredHost
}

// ServeHTTP upgrades the request to a WebSocket connection and relays raw
// bytes bidirectionally between it and the local sshd. Each WebSocket
// message is treated as an opaque chunk of the SSH byte stream - this
// matches how HTTP Injector-compatible clients frame the payload.
//
// The only gate applied here is the Host header matching this deployment's
// configured subdomain (when one is set); it is not a substitute for SSH
// authentication, which sshd still performs on every byte stream handed to
// it - this just stops the relay from being usable as an open WebSocket-to-
// anywhere-on-this-box proxy by a request aimed at an unrelated vhost.
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !hostAllowed(b.allowedHost, r.Host) {
		http.NotFound(w, r)
		return
	}

	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("bridge: websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sshConn, err := net.DialTimeout("tcp", b.sshTarget, 5*time.Second)
	if err != nil {
		log.Printf("bridge: failed to reach local sshd at %s: %v", b.sshTarget, err)
		return
	}
	defer sshConn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// ws -> sshd
	go func() {
		defer cancel()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if _, err := sshConn.Write(msg); err != nil {
				return
			}
		}
	}()

	// sshd -> ws
	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := sshConn.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("bridge: read from sshd failed: %v", err)
				}
				return
			}
		}
	}()

	<-ctx.Done()
}
