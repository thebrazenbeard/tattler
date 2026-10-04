package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func TestReadRecentRestoresEventsAcrossRotation(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 4; i++ {
		ev := model.Event{SchemaVersion: 1, ID: fmt.Sprintf("event-%d", i), Time: time.Unix(int64(i), 0).UTC(), Kind: "open", Source: "test"}
		if err := s.Append([]model.Event{ev}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	events, err := ReadRecent(dir, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events=%d want=3", len(events))
	}
	for i, want := range []string{"event-2", "event-3", "event-4"} {
		if events[i].ID != want {
			t.Fatalf("events[%d].ID=%q want=%q", i, events[i].ID, want)
		}
	}
}

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
