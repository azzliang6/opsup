package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// RDCleanPath v1 uses DER with EXPLICIT context-specific fields. The browser
// performs CredSSP; this gateway handles X.224 and the target TLS transport.
type rdpRequest struct {
	Version     int    `asn1:"explicit,tag:0"`
	Destination string `asn1:"explicit,tag:2,utf8"`
	ProxyAuth   string `asn1:"explicit,tag:3,utf8"`
	X224        []byte `asn1:"explicit,tag:6"`
}
type rdpResponse struct {
	Version      int      `asn1:"explicit,tag:0"`
	X224         []byte   `asn1:"explicit,tag:6"`
	Certificates [][]byte `asn1:"explicit,tag:7"`
	Address      string   `asn1:"explicit,tag:9,utf8"`
}

func parseRDPRequest(data []byte) (rdpRequest, error) {
	var req rdpRequest
	rest, err := asn1.Unmarshal(data, &req)
	if err != nil || len(rest) != 0 || req.Version != 3390 || req.ProxyAuth == "" {
		return req, fmt.Errorf("invalid RDCleanPath request")
	}
	// Unmarshal permits trailing SEQUENCE fields: canonical re-encoding rejects
	// them, as well as unsupported server_auth / PCB / VMConnect extensions.
	canonical, err := asn1.Marshal(req)
	if err != nil || string(canonical) != string(data) {
		return req, fmt.Errorf("unsupported RDCleanPath fields")
	}
	p := req.X224
	if len(p) < 19 || len(p) > 4096 || p[0] != 3 || p[1] != 0 || int(binary.BigEndian.Uint16(p[2:4])) != len(p) || int(p[4])+5 != len(p) || p[5] != 0xe0 {
		return req, fmt.Errorf("invalid X.224 request")
	}
	n := p[len(p)-8:]
	if n[0] != 1 || binary.LittleEndian.Uint16(n[2:4]) != 8 || binary.LittleEndian.Uint32(n[4:])&2 == 0 {
		return req, fmt.Errorf("RDP V1 requires NLA")
	}
	return req, nil
}

func rdpTLSConfig(server *models.Server) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: server.Host}
	if server.RDPCertFingerprint == "" {
		return cfg, nil
	}
	expected, err := hex.DecodeString(server.RDPCertFingerprint)
	if err != nil || len(expected) != sha256.Size {
		return nil, fmt.Errorf("invalid certificate pin")
	}
	// Explicit pin validation replaces PKI verification for self-signed hosts.
	cfg.InsecureSkipVerify = true
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return fmt.Errorf("missing RDP certificate")
		}
		actual := sha256.Sum256(cs.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
			return fmt.Errorf("RDP certificate fingerprint mismatch")
		}
		return nil
	}
	return cfg, nil
}

func negotiateRDP(ctx context.Context, raw net.Conn, request []byte, cfg *tls.Config) (*tls.Conn, []byte, error) {
	_ = raw.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err := io.Copy(raw, bytes.NewReader(request)); err != nil {
		return nil, nil, err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(raw, header); err != nil {
		return nil, nil, err
	}
	size := int(binary.BigEndian.Uint16(header[2:]))
	if header[0] != 3 || header[1] != 0 || size != 19 {
		return nil, nil, fmt.Errorf("invalid RDP negotiation response")
	}
	response := make([]byte, size)
	copy(response, header)
	if _, err := io.ReadFull(raw, response[4:]); err != nil {
		return nil, nil, err
	}
	selected := binary.LittleEndian.Uint32(response[15:19])
	if response[4] != 14 || response[5] != 0xd0 || response[11] != 2 || binary.LittleEndian.Uint16(response[13:15]) != 8 || (selected != 2 && selected != 8) {
		return nil, nil, fmt.Errorf("server must support TLS with NLA")
	}
	secured := tls.Client(raw, cfg)
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("RDP TLS certificate or handshake failed: %w", err)
	}
	_ = secured.SetDeadline(time.Time{})
	return secured, response, nil
}

func RDPHandler(cfg AppConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("serverId"), 10, 64)
		if err != nil || id <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		fail := func(reason string) {
			_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, reason), time.Now().Add(wsWriteTimeout))
		}
		ws.SetReadLimit(64 * 1024)
		_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
		kind, data, err := ws.ReadMessage()
		if err != nil || kind != websocket.BinaryMessage {
			fail("Expected RDCleanPath binary handshake")
			return
		}
		req, err := parseRDPRequest(data)
		if err != nil {
			fail("Invalid RDCleanPath handshake")
			return
		}
		if _, err := auth.ValidateToken(req.ProxyAuth, cfg.JWTSecret); err != nil {
			fail("Authentication required")
			return
		}
		server, err := models.GetServer(getDB(c), id)
		if err != nil || server == nil || server.Protocol != "rdp" || server.JumpServerID != nil {
			fail("RDP server unavailable")
			return
		}
		// Never use the client-supplied destination for dialing. Authorization binds
		// the route to a managed record. Credentials are never accepted or persisted.
		address := net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
		tlsCfg, err := rdpTLSConfig(server)
		if err != nil {
			fail("Invalid RDP certificate configuration")
			return
		}
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		raw, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
		if err != nil {
			fail("RDP server unreachable")
			return
		}
		defer raw.Close()
		stop := context.AfterFunc(ctx, func() { _ = raw.Close(); _ = ws.Close() })
		defer stop()
		conn, x224, err := negotiateRDP(ctx, raw, req.X224, tlsCfg)
		if err != nil {
			fail("RDP negotiation or certificate verification failed")
			return
		}
		defer conn.Close()
		certs := make([][]byte, 0)
		for _, cert := range conn.ConnectionState().PeerCertificates {
			certs = append(certs, cert.Raw)
		}
		host, _, _ := net.SplitHostPort(raw.RemoteAddr().String())
		response, err := asn1.Marshal(rdpResponse{3390, x224, certs, host})
		if err != nil {
			return
		}
		writer := &wsWriter{conn: ws}
		if err := writer.write(websocket.BinaryMessage, response); err != nil {
			return
		}
		relayRDP(ctx, cancel, ws, conn, writer)
	}
}

func relayRDP(ctx context.Context, cancel context.CancelFunc, ws *websocket.Conn, conn net.Conn, writer *wsWriter) {
	ws.SetReadLimit(4 * 1024 * 1024)
	_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		for {
			kind, data, err := ws.ReadMessage()
			if err != nil || kind != websocket.BinaryMessage {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
			if _, err := io.Copy(conn, bytes.NewReader(data)); err != nil {
				return
			}
		}
	}()
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if writer.write(websocket.PingMessage, nil) != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); _ = conn.Close(); _ = ws.Close(); <-done; <-pingDone }()
	buffer := make([]byte, 64*1024)
	for {
		n, err := conn.Read(buffer)
		if n > 0 {
			if writer.write(websocket.BinaryMessage, buffer[:n]) != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
