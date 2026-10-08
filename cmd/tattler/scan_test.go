package main

import (
	"errors"
	"github.com/thebrazenbeard/tattler/internal/model"
	"github.com/thebrazenbeard/tattler/internal/server"
	"github.com/thebrazenbeard/tattler/internal/tracker"
	"net/netip"
	"testing"
	"time"
)

func TestNetworkScanRetainsLastCompleteEvidenceOnPartial(t *testing.T) {
	tr := tracker.New()
	state := server.NewState(20)
	scan := &networkScan{tracker: tr, state: state}
	c := model.Connection{Kind: model.ObservationTCPSession, Protocol: "tcp",
		Local:  netip.MustParseAddrPort("127.0.0.1:2222"),
		Remote: netip.MustParseAddrPort("127.0.0.1:443"), Process: model.ProcessInfo{PID: 123}}
	at := time.Now().UTC()
	if opened, _ := scan.Accept([]model.Connection{c}, errors.New("partial"), at); len(opened) != 0 {
		t.Fatal("partial initial scan opened")
	}
	if scan.initialized {
		t.Fatal("baseline initialized from partial scan")
	}
	if opened, _ := scan.Accept([]model.Connection{c}, nil, at.Add(time.Second)); len(opened) != 0 {
		t.Fatal("first complete scan generated events")
	}
	if opened, closed := scan.Accept(nil, errors.New("partial"), at.Add(2*time.Second)); len(opened) != 0 || len(closed) != 0 {
		t.Fatal("partial scan generated close")
	}
	if len(tr.Current()) != 1 {
		t.Fatal("partial scan lost last complete state")
	}
	if opened, closed := scan.Accept([]model.Connection{c}, nil, at.Add(3*time.Second)); len(opened) != 0 || len(closed) != 0 {
		t.Fatal("recovery generated phantom churn")
	}
	if opened, closed := scan.Accept(nil, nil, at.Add(4*time.Second)); len(opened) != 0 || len(closed) != 1 {
		t.Fatal("genuine complete disappearance not emitted")
	}
}
