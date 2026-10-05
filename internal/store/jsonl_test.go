package store

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

type flushFailWriter struct{}

func (flushFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced flush failure")
}

func TestCloseReturnsFlushFailure(t *testing.T) {
	s, err := Open(t.TempDir(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	s.writer = bufio.NewWriterSize(flushFailWriter{}, 64)
	if _, err := s.writer.WriteString("pending"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err == nil {
		t.Fatal("Close succeeded despite buffered flush failure")
	}
}

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

func TestReadRecentStopsAfterWindowIsFilled(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl.2"), []byte("not-json\\n"), 0600); err != nil {
		t.Fatal(err)
	}

	events, err := ReadRecent(dir, 4, 2)
	if err != nil {
		t.Fatalf("older rotation should not be read after recent window is full: %v", err)
	}
	if len(events) != 2 || events[0].ID != "event-3" || events[1].ID != "event-4" {
		t.Fatalf("events=%+v", events)
	}
}

func TestAppendReportsRotationRenameFailure(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := os.Mkdir(s.path+".1", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.path+".1", "blocker"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	ev := model.Event{SchemaVersion: 1, ID: "event-1", Time: time.Unix(1, 0).UTC(), Kind: "open", Source: "test"}
	err = s.Append([]model.Event{ev})
	if err == nil {
		t.Fatal("Append succeeded despite rotation rename failure")
	}
	if _, statErr := os.Stat(s.path); statErr != nil {
		t.Fatalf("current journal was not preserved after failed rotation: %v", statErr)
	}
	if err := os.RemoveAll(s.path + ".1"); err != nil {
		t.Fatal(err)
	}
	retry := model.Event{SchemaVersion: 1, ID: "event-2", Time: time.Unix(2, 0).UTC(), Kind: "open", Source: "test"}
	if err := s.Append([]model.Event{retry}); err != nil {
		t.Fatalf("journal did not recover after rotation failure: %v", err)
	}
}

func TestRotationRetainsNewestEventsInOrder(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, 1, 2)
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

	events, err := ReadRecent(dir, 2, 3)
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
