package tracker

import (
	"net/netip"
	"strings"
	"sync"

	"github.com/thebrazenbeard/tattler/internal/model"
)

type Tracker struct {
	mu    sync.RWMutex
	state map[string]model.Connection
}

func New() *Tracker { return &Tracker{state: make(map[string]model.Connection)} }

func (t *Tracker) Baseline(conns []model.Connection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = make(map[string]model.Connection, len(conns))
	for _, c := range conns {
		t.state[c.Key()] = c
	}
}

func (t *Tracker) Diff(conns []model.Connection) (opened, closed []model.Connection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	next := make(map[string]model.Connection, len(conns))
	for _, c := range conns {
		k := c.Key()
		next[k] = c
		if _, ok := t.state[k]; !ok {
			opened = append(opened, c)
		}
	}
	for k, c := range t.state {
		if _, ok := next[k]; !ok {
			closed = append(closed, c)
		}
	}
	t.state = next
	return
}

func (t *Tracker) Current() []model.Connection {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]model.Connection, 0, len(t.state))
	for _, c := range t.state {
		out = append(out, c)
	}
	return out
}

func Classify(c model.Connection, listeners []model.Connection, local map[netip.Addr]struct{}) string {
	if _, ok := local[c.Local.Addr()]; !ok && !c.Local.Addr().IsLoopback() {
		return "unknown"
	}
	if strings.HasPrefix(c.Protocol, "udp") {
		return "outbound"
	}
	for _, l := range listeners {
		if l.Local.Port() != c.Local.Port() {
			continue
		}
		if l.Local.Addr().IsUnspecified() || l.Local.Addr() == c.Local.Addr() {
			return "inbound"
		}
	}
	return "outbound"
}
