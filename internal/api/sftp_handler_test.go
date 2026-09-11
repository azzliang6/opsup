package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestUploadCapsBodyBeforeMultipartParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prefix := "--upload\r\nContent-Disposition: form-data; name=\"file\"; filename=\"large.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n"
	body := io.MultiReader(strings.NewReader(prefix), io.LimitReader(zeroReader{}, maxUploadBody+1), strings.NewReader("\r\n--upload--\r\n"))
	request := httptest.NewRequest(http.MethodPost, "/servers/1/upload", body)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=upload")
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = request
	// No DB is installed. An oversized request must fail before connecting.
	SFTPUpload("")(ctx)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d: %s", response.Code, response.Body.String())
	}
}

type deadlineWriter struct {
	*httptest.ResponseRecorder
	readDeadline  time.Time
	writeDeadline time.Time
}

func (w *deadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.readDeadline = deadline
	return nil
}
func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	return nil
}

func TestSFTPOperationBoundsAndRestoresHTTPDeadlines(t *testing.T) {
	writer := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	cancel := sftpOperation(ctx)
	deadline, ok := ctx.Request.Context().Deadline()
	if !ok || time.Until(deadline) > sftpOperationTimeout || writer.readDeadline != deadline || writer.writeDeadline != deadline {
		t.Fatal("request and transport deadlines not installed")
	}
	cancel()
	if ctx.Request.Context().Err() == nil || !writer.readDeadline.IsZero() || !writer.writeDeadline.IsZero() {
		t.Fatal("operation cleanup did not restore deadlines")
	}
}
