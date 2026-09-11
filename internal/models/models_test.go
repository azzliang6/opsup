package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/azzliang6/opsup/internal/database"
)

func TestOnlyOneFirstAdministrator(t *testing.T) {
	db, err := database.Init(filepath.Join(t.TempDir(), "opsup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var succeeded atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := CreateFirstUser(db, fmt.Sprintf("admin%d", i), "hash")
			if err == nil {
				succeeded.Add(1)
			} else if !errors.Is(err, ErrAlreadyInitialized) {
				t.Errorf("setup: %v", err)
			}
		}(i)
	}
	wg.Wait()
	count, err := CountUsers(db)
	if err != nil || count != 1 || succeeded.Load() != 1 {
		t.Fatalf("users=%d succeeded=%d err=%v", count, succeeded.Load(), err)
	}
}

func TestServerRoundTripDoesNotSerializeCredentials(t *testing.T) {
	db, err := database.Init(filepath.Join(t.TempDir(), "opsup.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &Server{Name: "test", Host: "localhost", Port: 22, Username: "user", AuthType: "key", HostKey: "SHA256:fingerprint", PrivateKey: "encrypted-key", Password: "encrypted-password"}
	if err := CreateServer(db, s); err != nil {
		t.Fatal(err)
	}
	stored, err := GetServer(db, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.HostKey != s.HostKey || stored.PrivateKey != s.PrivateKey {
		t.Fatal("server data lost")
	}
	stored.HostKey = "SHA256:changed"
	if err := UpdateServer(db, stored); err != nil {
		t.Fatal(err)
	}
	list, err := ListServers(db)
	if err != nil || len(list) != 1 || list[0].HostKey != stored.HostKey {
		t.Fatalf("list: %#v %v", list, err)
	}
	for _, value := range []any{stored, stored.ToListItem(), list} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "encrypted-") || strings.Contains(string(encoded), "private_key") || strings.Contains(string(encoded), `"password":`) {
			t.Fatalf("credentials exposed: %s", encoded)
		}
	}
}
