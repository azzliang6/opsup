package sftpclient

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
)

type closeFailureWriter struct{ sftp.FileWriter }

func (w closeFailureWriter) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	writer, err := w.FileWriter.Filewrite(request)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(path.Base(request.Filepath), ".opsup-upload-") {
		return closeFailureFile{writer}, nil
	}
	return writer, nil
}

type closeFailureFile struct{ io.WriterAt }

func (f closeFailureFile) Close() error {
	if closer, ok := f.WriterAt.(io.Closer); ok {
		_ = closer.Close()
	}
	return errors.New("disk flush failed")
}

func TestUploadChecksRemoteCloseBeforePublish(t *testing.T) {
	client := newTestClient(t, "close-error")
	initial, err := client.sftpClient.OpenFile("/existing", os.O_WRONLY|os.O_CREATE)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := initial.Write([]byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	err = client.Upload("/existing", strings.NewReader("replacement"))
	if err == nil || !strings.Contains(err.Error(), "close upload") {
		t.Fatalf("ignored close error: %v", err)
	}
	file, err := client.Open("/existing")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil || string(content) != "original" {
		t.Fatalf("destination changed: %q %v", content, err)
	}
	files, err := client.ReadDir("/")
	if err != nil || len(files) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", files, err)
	}
}

func TestUploadWithoutPosixRenameRejectsOverwrite(t *testing.T) {
	if err := sftp.SetSFTPExtensions(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sftp.SetSFTPExtensions("hardlink@openssh.com", "posix-rename@openssh.com", "statvfs@openssh.com")
	})
	client := newTestClient(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, "existing")
	if err := os.WriteFile(dest, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(dest, strings.NewReader("replacement")); err == nil {
		t.Fatal("unsafe overwrite accepted")
	}
	content, err := os.ReadFile(dest)
	if err != nil || string(content) != "original" {
		t.Fatalf("destination changed: %q %v", content, err)
	}
	newPath := filepath.Join(dir, "new")
	if err := client.Upload(newPath, strings.NewReader("new data")); err != nil {
		t.Fatalf("new file upload failed: %v", err)
	}
	content, err = os.ReadFile(newPath)
	if err != nil || string(content) != "new data" {
		t.Fatalf("new file missing: %q %v", content, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("temporary file leaked: %v, %v", files, err)
	}
}
