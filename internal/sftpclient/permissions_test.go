package sftpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadPreservesPermissionsAndRejectsSymlinks(t *testing.T) {
	client := newTestClient(t)
	dir := t.TempDir()
	for _, mode := range []os.FileMode{0600, 0750} {
		dest := filepath.Join(dir, "existing")
		if err := os.WriteFile(dest, []byte("old"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dest, mode); err != nil {
			t.Fatal(err)
		}
		if err := client.Upload(dest, strings.NewReader("new")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(dest)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("changed permissions: %v %v", info, err)
		}
	}
	fresh := filepath.Join(dir, "fresh")
	if err := client.Upload(fresh, strings.NewReader("private")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fresh)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("new file is not private: %v %v", info, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(fresh, link); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(link, strings.NewReader("overwrite")); err == nil {
		t.Fatal("symlink overwritten")
	}
}
