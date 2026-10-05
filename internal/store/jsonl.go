package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/thebrazenbeard/tattler/internal/model"
)

type JSONL struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	keep     int
	file     *os.File
	writer   *bufio.Writer
	size     int64
}

func Open(dir string, maxBytes int64, keep int) (*JSONL, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &JSONL{path: filepath.Join(dir, "events.jsonl"), maxBytes: maxBytes, keep: keep}
	if err := s.open(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONL) open() error {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	s.file, s.writer, s.size = f, bufio.NewWriterSize(f, 64*1024), st.Size()
	return nil
}

func (s *JSONL) Append(events []model.Event) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var payload []byte
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		payload = append(payload, b...)
		payload = append(payload, '\n')
	}
	if s.maxBytes > 0 && s.size+int64(len(payload)) > s.maxBytes {
		if err := s.rotate(); err != nil {
			return err
		}
	}
	n, err := s.writer.Write(payload)
	s.size += int64(n)
	if err != nil {
		return err
	}
	return s.writer.Flush()
}

func (s *JSONL) rotate() error {
	if s.writer != nil {
		if err := s.writer.Flush(); err != nil {
			return fmt.Errorf("flush journal before rotation: %w", err)
		}
	}
	if s.file != nil {
		f := s.file
		s.file, s.writer = nil, nil
		if err := f.Close(); err != nil {
			if reopenErr := s.open(); reopenErr != nil {
				return errors.Join(
					fmt.Errorf("close journal before rotation: %w", err),
					fmt.Errorf("reopen journal after close failure: %w", reopenErr),
				)
			}
			return fmt.Errorf("close journal before rotation: %w", err)
		}
	}

	reopenAfterFailure := func(cause error) error {
		if reopenErr := s.open(); reopenErr != nil {
			return errors.Join(cause, fmt.Errorf("reopen journal after rotation failure: %w", reopenErr))
		}
		return cause
	}

	if s.keep <= 0 {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return reopenAfterFailure(fmt.Errorf("remove current journal during rotation: %w", err))
		}
		return s.open()
	}

	type move struct {
		from string
		to   string
	}
	var moved []move
	rollback := func(cause error, backup string, hadBackup bool) error {
		var errs []error
		errs = append(errs, cause)
		for i := len(moved) - 1; i >= 0; i-- {
			if err := os.Rename(moved[i].to, moved[i].from); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, fmt.Errorf("rollback %s -> %s: %w", filepath.Base(moved[i].to), filepath.Base(moved[i].from), err))
			}
		}
		if hadBackup {
			oldest := fmt.Sprintf("%s.%d", s.path, s.keep)
			if err := os.Rename(backup, oldest); err != nil {
				errs = append(errs, fmt.Errorf("restore oldest journal after failed rotation: %w", err))
			}
		}
		if reopenErr := s.open(); reopenErr != nil {
			errs = append(errs, fmt.Errorf("reopen journal after failed rotation: %w", reopenErr))
		}
		return errors.Join(errs...)
	}

	backup := s.path + ".rotate-oldest"
	if _, err := os.Stat(backup); err == nil {
		return reopenAfterFailure(fmt.Errorf("rotation recovery file already exists: %s", filepath.Base(backup)))
	} else if !errors.Is(err, os.ErrNotExist) {
		return reopenAfterFailure(fmt.Errorf("inspect rotation recovery file: %w", err))
	}

	oldest := fmt.Sprintf("%s.%d", s.path, s.keep)
	hadBackup := false
	if info, err := os.Stat(oldest); err == nil {
		if !info.Mode().IsRegular() {
			return reopenAfterFailure(fmt.Errorf("oldest journal path is not a regular file: %s", filepath.Base(oldest)))
		}
		if err := os.Rename(oldest, backup); err != nil {
			return reopenAfterFailure(fmt.Errorf("stage oldest journal for rotation: %w", err))
		}
		hadBackup = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return reopenAfterFailure(fmt.Errorf("inspect oldest journal before rotation: %w", err))
	}

	for i := s.keep - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", s.path, i)
		next := fmt.Sprintf("%s.%d", s.path, i+1)
		if err := os.Rename(old, next); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return rollback(fmt.Errorf("rotate %s -> %s: %w", filepath.Base(old), filepath.Base(next), err), backup, hadBackup)
		}
		moved = append(moved, move{from: old, to: next})
	}

	first := s.path + ".1"
	if err := os.Rename(s.path, first); err != nil {
		return rollback(fmt.Errorf("rotate current journal -> %s: %w", filepath.Base(first), err), backup, hadBackup)
	}
	moved = append(moved, move{from: s.path, to: first})

	if err := s.open(); err != nil {
		return rollback(fmt.Errorf("open new journal after rotation: %w", err), backup, hadBackup)
	}
	if hadBackup {
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("remove staged oldest journal after rotation: %w", err)
		}
	}
	return nil
}

func ReadRecent(dir string, keep, limit int) ([]model.Event, error) {
	if limit <= 0 {
		return nil, nil
	}
	if keep < 0 {
		keep = 0
	}

	base := filepath.Join(dir, "events.jsonl")
	out := make([]model.Event, 0, limit)
	var errs []error
	for i := 0; i <= keep && len(out) < limit; i++ {
		path := base
		if i > 0 {
			path = fmt.Sprintf("%s.%d", base, i)
		}
		events, err := readRecentFile(path, limit-len(out))
		if err != nil {
			errs = append(errs, err)
		}
		if len(events) == 0 {
			continue
		}
		merged := make([]model.Event, 0, len(events)+len(out))
		merged = append(merged, events...)
		merged = append(merged, out...)
		out = merged
	}
	return out, errors.Join(errs...)
}

func readRecentFile(path string, limit int) ([]model.Event, error) {
	if limit <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	ring := make([]model.Event, limit)
	count, next := 0, 0
	var errs []error
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var ev model.Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			errs = append(errs, fmt.Errorf("read %s line %d: %w", filepath.Base(path), line, err))
			continue
		}
		ring[next] = ev
		next = (next + 1) % limit
		if count < limit {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Errorf("scan %s: %w", filepath.Base(path), err))
	}
	if count == 0 {
		return nil, errors.Join(errs...)
	}

	out := make([]model.Event, count)
	start := 0
	if count == limit {
		start = next
	}
	for i := 0; i < count; i++ {
		out[i] = ring[(start+i)%limit]
	}
	return out, errors.Join(errs...)
}

func (s *JSONL) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writer != nil {
		_ = s.writer.Flush()
	}
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
