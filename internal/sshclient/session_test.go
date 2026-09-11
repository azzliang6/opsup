package sshclient

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestSessionContextClosesLiveTransport(t *testing.T) {
	peer := startPeer(t, "")
	client, err := Dial(context.Background(), peer.config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := NewSession(ctx, client, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	finished := make(chan struct{})
	go func() { _, _ = session.Stdout().Read(make([]byte, 1)); close(finished) }()
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled session read remained blocked")
	}
	awaitClosed(t, peer.closed)
}

func TestJumpSessionCloseUnblocksWrite(t *testing.T) {
	target := startPeer(t, "")
	jump := startPeer(t, "")
	client, err := DialViaJump(context.Background(), jump.config, target.config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := NewSession(context.Background(), client, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	writeDone := make(chan struct{})
	go func() { _ = session.Write(bytes.Repeat([]byte("x"), 8*1024*1024)); close(writeDone) }()
	select {
	case <-writeDone:
		t.Fatal("test requires blocked write")
	case <-time.After(100 * time.Millisecond):
	}
	closeDone := make(chan struct{})
	go func() { session.Close(); close(closeDone) }()
	awaitClosed(t, closeDone)
	awaitClosed(t, writeDone)
	awaitClosed(t, target.closed)
	awaitClosed(t, jump.closed)
}
