package api

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/gorilla/websocket"
)

var testX224 = []byte{3, 0, 0, 19, 14, 0xe0, 0, 0, 0, 0, 0, 1, 0, 8, 0, 3, 0, 0, 0}

func TestRDPRequestStrictParsing(t *testing.T) {
	req := rdpRequest{3390, "ignored:3389", "token", testX224}
	good, err := asn1.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseRDPRequest(good); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, good[:len(good)-1], append(append([]byte{}, good...), 0)} {
		if _, err := parseRDPRequest(data); err == nil {
			t.Fatal("accepted malformed request")
		}
	}
	req.X224 = append([]byte{}, testX224...)
	req.X224[15] = 1
	bad, _ := asn1.Marshal(req)
	if _, err := parseRDPRequest(bad); err == nil {
		t.Fatal("accepted non-NLA request")
	}
	// Unknown trailing sequence fields must not be silently ignored by Go ASN.1.
	bad = append(append([]byte{}, good...), 0xa9, 3, 0x0c, 1, 'x')
	bad[1] += 5
	if _, err := parseRDPRequest(bad); err == nil {
		t.Fatal("accepted unsupported extension")
	}
}

func TestRDPResponseUpstreamDER(t *testing.T) {
	// Wire fixture from IronRDP's rdcleanpath conformance test.
	p := []byte{0xde, 0xad, 0xbe, 0xff}
	encoded, err := asn1.Marshal(rdpResponse{3390, p, [][]byte{p, p, p}, "192.168.7.95"})
	if err != nil {
		t.Fatal(err)
	}
	expected := "3034a00402020d3ea6060404deadbeffa71430120404deadbeff0404deadbeff0404deadbeffa90e0c0c3139322e3136382e372e3935"
	if hex.EncodeToString(encoded) != expected {
		t.Fatalf("incompatible DER: %x", encoded)
	}
}

func TestRDPCertificatePolicy(t *testing.T) {
	cert := &x509.Certificate{Raw: []byte("certificate")}
	sum := sha256.Sum256(cert.Raw)
	cfg, err := rdpTLSConfig(&models.Server{Host: "host"})
	if err != nil || cfg.InsecureSkipVerify || cfg.ServerName != "host" {
		t.Fatal("PKI validation must be the default")
	}
	cfg, err = rdpTLSConfig(&models.Server{Host: "host", RDPCertFingerprint: hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}); err != nil {
		t.Fatal(err)
	}
	if err := cfg.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Raw: []byte("other")}}}); err == nil {
		t.Fatal("accepted mismatching pin")
	}
	if err := cfg.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Fatal("accepted no certificate")
	}
}

func TestRDPServerAPINeverStoresCredentials(t *testing.T) {
	r, cfg := securityRouter(t)
	token, _, _ := auth.GenerateToken(1, "admin", cfg.JWTSecret)
	body := `{"protocol":"rdp","name":"Windows","host":"localhost","username":"Administrator","rdp_domain":"EXAMPLE"}`
	res := performRequest(r, "POST", "/api/servers", body, token)
	if res.Code != 201 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	s, err := models.GetServer(cfg.DB, 1)
	if err != nil || s.Port != 3389 || s.Protocol != "rdp" || s.RDPDomain != "EXAMPLE" || s.Password != "" || s.PrivateKey != "" {
		t.Fatalf("unexpected record: %+v %v", s, err)
	}
	for _, extra := range []string{`"password":"secret"`, `"private_key":"key"`, `"copy_key_from":1`, `"jump_server_id":1`, `"rdp_cert_fingerprint":"bad"`, `"port":65536`} {
		res = performRequest(r, "POST", "/api/servers", strings.TrimSuffix(body, "}")+","+extra+"}", token)
		if res.Code != 400 {
			t.Fatalf("accepted %s: %d", extra, res.Code)
		}
	}
	if _, err := connectionConfig(s, cfg.EncryptionKey); err == nil {
		t.Fatal("RDP allowed through SSH/SFTP")
	}
	// Switching a saved SSH record to RDP must delete old encrypted credentials.
	if _, err := cfg.DB.Exec("UPDATE servers SET protocol='ssh', password='old', private_key='old' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	res = performRequest(r, "PUT", "/api/servers/1", body, token)
	if res.Code != 200 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	s, _ = models.GetServer(cfg.DB, 1)
	if s.Password != "" || s.PrivateKey != "" {
		t.Fatal("retained credentials after protocol switch")
	}
}

func TestRDPEndToEndGateway(t *testing.T) {
	r, cfg := securityRouter(t)
	// Reuse httptest's self-signed certificate in a minimal RDP TLS peer.
	certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificate := certServer.TLS.Certificates[0]
	certServer.Close()
	hash := sha256.Sum256(certificate.Certificate[0])
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	peerDone := make(chan error, 1)
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer raw.Close()
		_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
		request := make([]byte, len(testX224))
		if _, err = io.ReadFull(raw, request); err != nil {
			peerDone <- err
			return
		}
		if !bytes.Equal(request, testX224) {
			peerDone <- io.ErrUnexpectedEOF
			return
		}
		response := append([]byte{}, testX224...)
		response[5] = 0xd0
		response[11] = 2
		response[15] = 2
		if _, err = raw.Write(response); err != nil {
			peerDone <- err
			return
		}
		secured := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
		if err = secured.Handshake(); err != nil {
			peerDone <- err
			return
		}
		data := make([]byte, 4)
		if _, err = io.ReadFull(secured, data); err == nil {
			_, err = secured.Write(data)
		}
		if err != nil {
			peerDone <- err
			return
		}
		// The gateway must release the target when its browser socket closes.
		_, err = secured.Read(data)
		if err == nil {
			peerDone <- io.ErrNoProgress
			return
		}
		if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
			peerDone <- err
			return
		}
		peerDone <- nil
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := &models.Server{Name: "RDP", Host: host, Port: p, Username: "admin", Protocol: "rdp", RDPCertFingerprint: hex.EncodeToString(hash[:])}
	if err := models.CreateServer(cfg.DB, s); err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(r)
	defer httpServer.Close()
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rdp/" + strconv.FormatInt(s.ID, 10)
	// Reject cross-origin upgrades before opening a target connection.
	ws, res, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{"https://untrusted.invalid"}})
	if err == nil {
		ws.Close()
		t.Fatal("accepted cross-origin websocket")
	}
	if res.StatusCode != 403 {
		t.Fatal(res.StatusCode)
	}
	ws, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	invalid, _ := asn1.Marshal(rdpRequest{3390, "127.0.0.1:1", "invalid", testX224})
	_ = ws.WriteMessage(websocket.BinaryMessage, invalid)
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err = ws.ReadMessage(); err == nil {
		t.Fatal("accepted invalid JWT")
	}
	ws.Close()
	ws, _, err = websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	token, _, _ := auth.GenerateToken(1, "admin", cfg.JWTSecret)
	// A hostile destination is ignored; only the saved server endpoint is dialed.
	request, _ := asn1.Marshal(rdpRequest{3390, "127.0.0.1:1", token, testX224})
	if err = ws.WriteMessage(websocket.BinaryMessage, request); err != nil {
		t.Fatal(err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var response rdpResponse
	if _, err = asn1.Unmarshal(data, &response); err != nil || !bytes.Equal(response.Certificates[0], certificate.Certificate[0]) {
		t.Fatalf("bad handshake %v", err)
	}
	if err = ws.WriteMessage(websocket.BinaryMessage, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	_, data, err = ws.ReadMessage()
	if err != nil || string(data) != "ping" {
		t.Fatalf("relay: %q %v", data, err)
	}
	ws.Close()
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("target connection leaked")
	}
	// Ensure response metadata contains no proxy authentication material.
	out, _ := json.Marshal(s.ToListItem())
	if bytes.Contains(out, []byte(token)) {
		t.Fatal("leaked token")
	}
}
