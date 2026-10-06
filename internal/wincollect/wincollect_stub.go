//go:build !windows

package wincollect

import (
	"errors"
	"time"

	"github.com/thebrazenbeard/tattler/internal/metrics"
	"github.com/thebrazenbeard/tattler/internal/model"
)

type Collector struct{}

func New() *Collector { return &Collector{} }

func (c *Collector) Connections() ([]model.Connection, error) {
	return nil, errors.New("Windows collector unavailable on this platform")
}

func (c *Collector) Sample(time.Time) (metrics.Sample, error) {
	return metrics.Sample{}, errors.New("Windows collector unavailable on this platform")
}
