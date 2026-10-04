package main

import (
	"net/netip"
	"testing"

	"github.com/thebrazenbeard/tattler/internal/model"
	"github.com/thebrazenbeard/tattler/internal/procnet"
)

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
