package tracker

import (
	"net/netip"
	"testing"

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
