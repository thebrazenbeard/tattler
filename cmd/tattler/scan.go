package main

import (
	"github.com/thebrazenbeard/tattler/internal/model"
	"github.com/thebrazenbeard/tattler/internal/server"
	"github.com/thebrazenbeard/tattler/internal/tracker"
	"time"
)

// networkScan accepts only complete collector passes. Partial results cannot
// justify absence/closure. A first complete scan establishes an event-free baseline.
type networkScan struct {
	tracker     *tracker.Tracker
	state       *server.State
	initialized bool
}

func (s *networkScan) Accept(rows []model.Connection, scanErr error, at time.Time) (opened, closed []model.Connection) {
	s.state.NoteNetworkScan(at, scanErr == nil)
	if scanErr != nil {
		return nil, nil
	}
	if !s.initialized {
		s.tracker.BaselineAt(rows, at)
		s.initialized = true
	} else {
		opened, closed = s.tracker.DiffAt(rows, at)
	}
	s.state.SetCurrent(s.tracker.Annotate(rows))
	return
}
