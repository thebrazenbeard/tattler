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
