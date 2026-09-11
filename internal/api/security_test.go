package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/azzliang6/opsup/internal/auth"
	"github.com/azzliang6/opsup/internal/database"
	"github.com/azzliang6/opsup/internal/models"
	"github.com/gin-gonic/gin"
)

func securityRouter(t *testing.T) (*gin.Engine, AppConfig) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := database.Init(filepath.Join(t.TempDir(), "opsup.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := AppConfig{DB: db, JWTSecret: strings.Repeat("s", 32), EncryptionKey: strings.Repeat("ab", 32)}
	return SetupRouter(cfg), cfg
}

func performRequest(r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

func TestSetupLoginAndInitializationLock(t *testing.T) {
	r, _ := securityRouter(t)
	body := `{"username":"admin","password":"a-strong-test-password"}`
	if response := performRequest(r, "POST", "/api/auth/setup", body, ""); response.Code != 200 {
		t.Fatalf("setup: %d %s", response.Code, response.Body)
	}
	if response := performRequest(r, "POST", "/api/auth/setup", body, ""); response.Code != 403 {
		t.Fatalf("second setup: %d", response.Code)
	}
	if response := performRequest(r, "POST", "/api/auth/login", body, ""); response.Code != 200 {
		t.Fatalf("login: %d", response.Code)
	}
	if response := performRequest(r, "POST", "/api/auth/login", `{"username":"admin","password":"wrong"}`, ""); response.Code != 401 {
		t.Fatalf("bad password: %d", response.Code)
	}
}

func TestAuthenticationRateLimitAndProxySpoofing(t *testing.T) {
	r, _ := securityRouter(t)
	for i := 0; i < 11; i++ {
		request := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"none","password":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", "198.51.100."+string(rune('a'+i)))
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if i < 10 && response.Code != 401 {
			t.Fatalf("attempt %d: %d", i, response.Code)
		}
		if i == 10 && (response.Code != 429 || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limiter: %d", response.Code)
		}
	}
}

func TestServerResponsesAndHostKeyValidation(t *testing.T) {
	r, cfg := securityRouter(t)
	token, _, err := auth.GenerateToken(1, "admin", cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	payload := map[string]any{"name": "server", "host": "localhost", "username": "user", "auth_type": "password", "password": "remote-secret", "host_key": fingerprint}
	encoded, _ := json.Marshal(payload)
	response := performRequest(r, "POST", "/api/servers", string(encoded), token)
	if response.Code != 201 {
		t.Fatalf("create: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/api/servers", "/api/servers/1"} {
		response := performRequest(r, "GET", path, "", token)
		if response.Code != 200 || strings.Contains(response.Body.String(), `"password":`) || strings.Contains(response.Body.String(), "remote-secret") {
			t.Fatalf("unsafe response: %d %s", response.Code, response.Body)
		}
		if !strings.Contains(response.Body.String(), fingerprint) {
			t.Fatal("host key missing from response")
		}
	}
	stored, err := models.GetServer(cfg.DB, 1)
	if err != nil || stored.Password == "remote-secret" || stored.Password == "" {
		t.Fatalf("credential storage: %v", err)
	}
	payload["host_key"] = "not-a-fingerprint"
	encoded, _ = json.Marshal(payload)
	if response := performRequest(r, "PUT", "/api/servers/1", string(encoded), token); response.Code != 400 {
		t.Fatalf("bad fingerprint: %d", response.Code)
	}
	if response := performRequest(r, "GET", "/api/servers", "", ""); response.Code != 401 {
		t.Fatalf("unauthenticated: %d", response.Code)
	}
}
