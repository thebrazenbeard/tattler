package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
	"github.com/thebrazenbeard/tattler/internal/procnet"
	"github.com/thebrazenbeard/tattler/internal/server"
	"github.com/thebrazenbeard/tattler/internal/store"
)

func TestNewUIStateRestoresPersistedEvents(t *testing.T) {
	dir := t.TempDir()
	journal, err := store.Open(dir, 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := model.Event{
		SchemaVersion: 1,
		ID:            "persisted-event",
		Time:          time.Date(2026, 10, 4, 22, 0, 0, 0, time.UTC),
		Kind:          "open",
		Source:        "test",
	}
	if err := journal.Append([]model.Event{want}); err != nil {
		t.Fatal(err)
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}

	state := newUIState(dir, 4)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	(&server.Server{State: state}).Handler().ServeHTTP(rec, req)

	var got []model.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want.ID {
		t.Fatalf("events=%+v want persisted event %q", got, want.ID)
	}
}

func TestDecorateDirectionPersistsIntoEvents(t *testing.T) {
	localAddr := netip.MustParseAddr("192.168.1.187")
	cases := []struct {
		name      string
		localPort uint16
		listeners []model.Connection
		want      string
	}{
		{
			name:      "inbound",
			localPort: 8080,
			listeners: []model.Connection{{
				Protocol: "tcp",
				Local:    netip.MustParseAddrPort("0.0.0.0:8080"),
				State:    "LISTEN",
			}},
			want: "inbound",
		},
		{
			name:      "outbound",
			localPort: 55000,
			want:      "outbound",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := model.Connection{
				Protocol: "tcp",
				Local:    netip.AddrPortFrom(localAddr, tc.localPort),
				Remote:   netip.MustParseAddrPort("1.2.3.4:443"),
				State:    "ESTABLISHED",
				Inode:    9001,
			}
			got := decorate(
				procnet.Snapshot{Listeners: tc.listeners, Connections: []model.Connection{conn}},
				t.TempDir(),
				nil,
				map[netip.Addr]struct{}{localAddr: {}},
				map[uint64]model.ProcessInfo{},
			)
			if len(got) != 1 || got[0].Direction != tc.want {
				t.Fatalf("connections=%+v want direction=%q", got, tc.want)
			}
			ev := model.NewEvent(time.Date(2026, 10, 4, 23, 0, 0, 0, time.UTC), "test-host", "open", "proc-sampler", got[0])
			if ev.Connection.Direction != tc.want {
				t.Fatalf("event direction=%q want=%q", ev.Connection.Direction, tc.want)
			}
		})
	}
}

func TestDecorateAddsOwnerWithoutInventingProcess(t *testing.T) {
	conn := model.Connection{
		Protocol: "tcp",
		Local:    netip.MustParseAddrPort("192.168.1.187:32400"),
		Remote:   netip.MustParseAddrPort("192.168.1.20:50000"),
		State:    "ESTABLISHED",
		Inode:    1234,
		UID:      297536,
	}
	snap := procnet.Snapshot{Connections: []model.Connection{conn}}
	owners := map[uint32]string{297536: "PlexMediaServer"}
	got := decorate(snap, t.TempDir(), owners, map[netip.Addr]struct{}{}, map[uint64]model.ProcessInfo{})
	if len(got) != 1 {
		t.Fatalf("connections=%d", len(got))
	}
	if got[0].Owner != "PlexMediaServer" {
		t.Fatalf("owner=%q", got[0].Owner)
	}
	if got[0].Process.PID != 0 || got[0].Process.Name != "" || got[0].Process.Exe != "" {
		t.Fatalf("unexpected invented process attribution: %+v", got[0].Process)
	}
}

func TestDecoratePreservesExactProcessAndOwner(t *testing.T) {
	conn := model.Connection{
		Protocol: "tcp",
		Local:    netip.MustParseAddrPort("192.168.1.187:32400"),
		Remote:   netip.MustParseAddrPort("192.168.1.20:50000"),
		State:    "ESTABLISHED",
		Inode:    5678,
		UID:      297536,
	}
	exact := model.ProcessInfo{PID: 42, Name: "Plex Worker", Exe: "/plex/worker"}
	cache := map[uint64]model.ProcessInfo{5678: exact}
	got := decorate(
		procnet.Snapshot{Connections: []model.Connection{conn}},
		t.TempDir(),
		map[uint32]string{297536: "PlexMediaServer"},
		map[netip.Addr]struct{}{},
		cache,
	)
	if got[0].Owner != "PlexMediaServer" {
		t.Fatalf("owner=%q", got[0].Owner)
	}
	if got[0].Process != exact {
		t.Fatalf("process=%+v", got[0].Process)
	}
}
