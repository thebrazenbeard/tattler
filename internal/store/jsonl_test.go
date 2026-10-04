package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func TestAppendAndRotate(t *testing.T) {
	s, err := Open(t.TempDir(), 180, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 6; i++ {
		ev := model.Event{SchemaVersion: 1, Time: time.Unix(int64(i), 0).UTC(), Kind: "open", Source: "test"}
		if err := s.Append([]model.Event{ev}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(s.path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Clean(s.path + ".1")); err != nil {
		t.Fatal("rotation did not occur")
	}
}
