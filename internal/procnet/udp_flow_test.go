package procnet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func TestReadTableClassifiesConnectedUDPAsFlow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "udp")
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:C350 04030201:01BB 01 00000000:00000000 00:00000000 00000000  1000        0 42\n" +
		"   1: 00000000:14E9 00000000:0000 07 00000000:00000000 00:00000000 00000000  1000        0 43\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := readTable(path, "udp", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].Kind != model.ObservationUDPFlow {
		t.Fatalf("connected kind=%q", rows[0].Kind)
	}
	if rows[1].Kind != model.ObservationUDPEndpoint {
		t.Fatalf("unconnected kind=%q", rows[1].Kind)
	}
}
