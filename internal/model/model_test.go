package model

import (
	"encoding/json"
	"testing"
)

func TestConnectionKeyDistinguishesObservationKind(t *testing.T) {
	var session, endpoint Connection
	if err := json.Unmarshal([]byte(`{"kind":"tcp_session","protocol":"tcp","local":"127.0.0.1:9000","remote":"127.0.0.1:9001"}`), &session); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"kind":"tcp_listener","protocol":"tcp","local":"127.0.0.1:9000","remote":"127.0.0.1:9001"}`), &endpoint); err != nil {
		t.Fatal(err)
	}
	if session.Key() == endpoint.Key() {
		t.Fatalf("keys collapse distinct observation kinds: %q", session.Key())
	}
}

func TestConnectionKeyDistinguishesPIDWhenInodeUnavailable(t *testing.T) {
	a := Connection{Protocol: "udp", Process: ProcessInfo{PID: 101}}
	b := Connection{Protocol: "udp", Process: ProcessInfo{PID: 202}}
	if a.Key() == b.Key() {
		t.Fatalf("keys collapse distinct PID-owned observations without inode: %q", a.Key())
	}
}
