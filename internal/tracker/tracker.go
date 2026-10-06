package tracker

import (
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/thebrazenbeard/tattler/internal/model"
)

type Tracker struct {
	mu    sync.RWMutex
	state map[string]model.Connection
}

func New() *Tracker { return &Tracker{state: make(map[string]model.Connection)} }

func (t *Tracker) Baseline(conns []model.Connection) {
	t.BaselineAt(conns, time.Now().UTC())
}

func (t *Tracker) BaselineAt(conns []model.Connection, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	at = at.UTC()
	t.state = make(map[string]model.Connection, len(conns))
	for _, c := range conns {
		c.FirstSeen = at
		t.state[c.Key()] = c
	}
}

func (t *Tracker) Diff(conns []model.Connection) (opened, closed []model.Connection) {
	return t.DiffAt(conns, time.Now().UTC())
}

func (t *Tracker) DiffAt(conns []model.Connection, at time.Time) (opened, closed []model.Connection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	at = at.UTC()
	next := make(map[string]model.Connection, len(conns))
	for _, c := range conns {
		k := c.Key()
		if previous, ok := t.state[k]; ok {
			c.FirstSeen = previous.FirstSeen
		} else {
			c.FirstSeen = at
			opened = append(opened, c)
		}
		next[k] = c
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

func (t *Tracker) Annotate(conns []model.Connection) []model.Connection {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]model.Connection, 0, len(conns))
	for _, c := range conns {
		if tracked, ok := t.state[c.Key()]; ok {
			c.FirstSeen = tracked.FirstSeen
		}
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
