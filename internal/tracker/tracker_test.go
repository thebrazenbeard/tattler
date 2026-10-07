package tracker

import (
	"net/netip"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func TestDiffAndDirection(t *testing.T) {
	tr := New()
	local := netip.MustParseAddr("192.168.1.187")
	conn := model.Connection{
		Protocol: "tcp",
		Local:    netip.MustParseAddrPort("192.168.1.187:8080"),
		Remote:   netip.MustParseAddrPort("1.2.3.4:50000"),
		Inode:    44,
	}
	listener := model.Connection{Protocol: "tcp", Local: netip.MustParseAddrPort("0.0.0.0:8080"), State: "LISTEN"}
	if got := Classify(conn, []model.Connection{listener}, map[netip.Addr]struct{}{local: {}}); got != "inbound" {
		t.Fatalf("direction=%s", got)
	}
	tr.Baseline(nil)
	opened, closed := tr.Diff([]model.Connection{conn})
	if len(opened) != 1 || len(closed) != 0 {
		t.Fatalf("open=%d close=%d", len(opened), len(closed))
	}
	opened, closed = tr.Diff(nil)
	if len(opened) != 0 || len(closed) != 1 {
		t.Fatalf("open=%d close=%d", len(opened), len(closed))
	}
}

func TestCurrentPreservesContinuousFirstSeenAge(t *testing.T) {
	tr := New()
	conn := model.Connection{
		Kind:     model.ObservationTCPSession,
		Protocol: "tcp",
		Local:    netip.MustParseAddrPort("127.0.0.1:12345"),
		Remote:   netip.MustParseAddrPort("127.0.0.1:443"),
		Process:  model.ProcessInfo{PID: 77},
	}
	start := time.Date(2026, 10, 6, 22, 0, 0, 0, time.UTC)
	tr.BaselineAt([]model.Connection{conn}, start)

	current := tr.Current()
	if len(current) != 1 || !current[0].FirstSeen.Equal(start) {
		t.Fatalf("current=%+v, want first_seen=%s", current, start)
	}

	later := start.Add(30 * time.Second)
	opened, closed := tr.DiffAt([]model.Connection{conn}, later)
	if len(opened) != 0 || len(closed) != 0 {
		t.Fatalf("stable observation emitted open=%d close=%d", len(opened), len(closed))
	}
	current = tr.Current()
	if !current[0].FirstSeen.Equal(start) {
		t.Fatalf("stable observation first_seen=%s, want %s", current[0].FirstSeen, start)
	}

	tr.DiffAt(nil, later.Add(time.Second))
	reopened := later.Add(2 * time.Second)
	tr.DiffAt([]model.Connection{conn}, reopened)
	current = tr.Current()
	if !current[0].FirstSeen.Equal(reopened) {
		t.Fatalf("reopened observation first_seen=%s, want %s", current[0].FirstSeen, reopened)
	}
}

func TestAnnotatePreservesDuplicateCollectorRows(t *testing.T) {
	tr := New()
	conn := model.Connection{
		Kind:     model.ObservationUDPEndpoint,
		Protocol: "udp",
		Local:    netip.MustParseAddrPort("0.0.0.0:5353"),
		Process:  model.ProcessInfo{PID: 99},
	}
	start := time.Date(2026, 10, 6, 22, 30, 0, 0, time.UTC)
	raw := []model.Connection{conn, conn}
	tr.BaselineAt(raw, start)

	annotated := tr.Annotate(raw)
	if len(annotated) != 2 {
		t.Fatalf("annotated len=%d, want raw collector cardinality 2", len(annotated))
	}
	for i, got := range annotated {
		if !got.FirstSeen.Equal(start) {
			t.Fatalf("annotated[%d].first_seen=%s, want %s", i, got.FirstSeen, start)
		}
	}
}

// Distinct collection rows can share a tracking identity. They must not
// create multiple events with the same deterministic event ID.
func TestDuplicateCollectorKeysEmitOneOpening(t *testing.T) {
	tr := New()
	c := model.Connection{Kind: model.ObservationUDPEndpoint, Protocol: "udp",
		Local: netip.MustParseAddrPort("0.0.0.0:5353"), Process: model.ProcessInfo{PID: 42}}
	at := time.Now().UTC()
	tr.BaselineAt(nil, at)
	opened, closed := tr.DiffAt([]model.Connection{c, c}, at.Add(time.Second))
	if len(opened) != 1 || len(closed) != 0 || len(tr.Current()) != 1 {
		t.Fatalf("opened=%d closed=%d current=%d", len(opened), len(closed), len(tr.Current()))
	}
	opened, closed = tr.DiffAt([]model.Connection{c, c}, at.Add(2*time.Second))
	if len(opened) != 0 || len(closed) != 0 {
		t.Fatalf("duplicate keys cause churn: %d/%d", len(opened), len(closed))
	}
	other := c
	other.Process.PID = 43
	opened, closed = tr.DiffAt([]model.Connection{c, other}, at.Add(3*time.Second))
	if len(opened) != 1 || len(closed) != 0 {
		t.Fatalf("distinct PID conflated: %d/%d", len(opened), len(closed))
	}
}
