package store

import (
	"bufio"
	"encoding/json"
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
		_ = s.writer.Flush()
	}
	if s.file != nil {
		_ = s.file.Close()
	}
	for i := s.keep - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", s.path, i)
		next := fmt.Sprintf("%s.%d", s.path, i+1)
		if i+1 > s.keep {
			_ = os.Remove(old)
			continue
		}
		_ = os.Rename(old, next)
	}
	if s.keep > 0 {
		_ = os.Rename(s.path, s.path+".1")
	} else {
		_ = os.Remove(s.path)
	}
	return s.open()
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
