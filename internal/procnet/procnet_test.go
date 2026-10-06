package procnet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTCP(t *testing.T) {
	root := t.TempDir()
	netDir := filepath.Join(root, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	header := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	rows := header +
		"   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 111 1\n" +
		"   1: BB01A8C0:1F90 04030201:C001 01 00000000:00000000 00:00000000 00000000 1024 0 222 1\n"
	if err := os.WriteFile(filepath.Join(netDir, "tcp"), []byte(rows), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Listeners) != 1 {
		t.Fatalf("listeners=%d", len(s.Listeners))
	}
	if len(s.Connections) != 1 {
		t.Fatalf("connections=%d", len(s.Connections))
	}
	c := s.Connections[0]
	if got := c.Local.String(); got != "192.168.1.187:8080" {
		t.Fatalf("local=%s", got)
	}
	if got := c.Remote.String(); got != "1.2.3.4:49153" {
		t.Fatalf("remote=%s", got)
	}
	if c.Inode != 222 || c.UID != 1024 || c.State != "ESTABLISHED" {
		t.Fatalf("bad row: %+v", c)
	}
}

func TestReadRetainsTypedListenersAndUnconnectedUDP(t *testing.T) {
	root := t.TempDir()
	netDir := filepath.Join(root, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	header := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	tcp := header +
		"   0: 00000000:238B 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 111 1\n" +
		"   1: 0100007F:C350 0100007F:01BB 01 00000000:00000000 00:00000000 00000000 1000 0 222 1\n"
	udp := header +
		"   0: 00000000:14E9 00000000:0000 07 00000000:00000000 00:00000000 00000000 1000 0 333 1\n"
	if err := os.WriteFile(filepath.Join(netDir, "tcp"), []byte(tcp), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(netDir, "udp"), []byte(udp), 0644); err != nil {
		t.Fatal(err)
	}
	snap, err := Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Listeners) != 1 {
		t.Fatalf("listeners=%d, want 1", len(snap.Listeners))
	}
	if snap.Listeners[0].Kind != "tcp_listener" {
		t.Fatalf("listener kind=%q", snap.Listeners[0].Kind)
	}
	if len(snap.Connections) != 2 {
		t.Fatalf("observations=%d, want TCP session + UDP endpoint", len(snap.Connections))
	}
	kinds := map[string]bool{}
	for _, c := range snap.Connections {
		kinds[c.Kind] = true
		if c.Kind == "udp_endpoint" {
			if got := c.Remote.String(); got != "0.0.0.0:0" {
				t.Fatalf("udp remote=%q, want unspecified peer", got)
			}
		}
	}
	if !kinds["tcp_session"] || !kinds["udp_endpoint"] {
		t.Fatalf("kinds=%v, want tcp_session and udp_endpoint", kinds)
	}
}
