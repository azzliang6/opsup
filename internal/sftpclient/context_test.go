package sftpclient

import (
	"context"
	"testing"
	"time"

	"github.com/azzliang6/opsup/internal/sshclient"
)

func TestCancelledOperationUnblocksSFTPRead(t *testing.T) {
	cfg, closed := sftpPeer(t, "stall-io")
	owner, err := sshclient.Dial(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := NewClient(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result := make(chan error, 1)
	go func() { _, err := client.ReadDir("/"); result <- err }()
	select {
	case <-result:
		t.Fatal("test requires blocked SFTP operation")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled read succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled SFTP operation remained blocked")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled SFTP leaked transport")
	}
}
