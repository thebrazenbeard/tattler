package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"time"
)

type ProcessInfo struct {
	PID  int    `json:"pid,omitempty"`
	Name string `json:"name,omitempty"`
	Exe  string `json:"exe,omitempty"`
}

type Connection struct {
	Protocol  string         `json:"protocol"`
	Local     netip.AddrPort `json:"local"`
	Remote    netip.AddrPort `json:"remote"`
	State     string         `json:"state,omitempty"`
	Direction string         `json:"direction,omitempty"`
	Inode     uint64         `json:"inode,omitempty"`
	UID       uint32         `json:"uid,omitempty"`
	Process   ProcessInfo    `json:"process,omitempty"`
}

func (c Connection) Key() string {
	return fmt.Sprintf("%s|%s|%s|%d", c.Protocol, c.Local, c.Remote, c.Inode)
}

type Event struct {
	SchemaVersion int        `json:"schema_version"`
	ID            string     `json:"event_id"`
	Time          time.Time  `json:"observed_at"`
	Host          string     `json:"host"`
	Kind          string     `json:"kind"`
	Connection    Connection `json:"connection"`
	Source        string     `json:"source"`
}

func NewEvent(at time.Time, host, kind, source string, c Connection) Event {
	at = at.UTC()
	raw := fmt.Sprintf("v1|%s|%s|%s|%s|%s", at.Format(time.RFC3339Nano), host, kind, source, c.Key())
	sum := sha256.Sum256([]byte(raw))
	return Event{SchemaVersion: 1, ID: hex.EncodeToString(sum[:]), Time: at, Host: host, Kind: kind, Connection: c, Source: source}
}
