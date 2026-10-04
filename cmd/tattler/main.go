package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/thebrazenbeard/tattler/internal/metrics"
	"github.com/thebrazenbeard/tattler/internal/model"
	"github.com/thebrazenbeard/tattler/internal/procmap"
	"github.com/thebrazenbeard/tattler/internal/procnet"
	"github.com/thebrazenbeard/tattler/internal/server"
	"github.com/thebrazenbeard/tattler/internal/store"
	"github.com/thebrazenbeard/tattler/internal/tracker"
	"github.com/thebrazenbeard/tattler/internal/uidmap"
)

func main() {
	var (
		listen          = flag.String("listen", "127.0.0.1:9147", "HTTP listen address")
		stateDir        = flag.String("state-dir", "./tattler-state", "persistent state directory")
		procRoot        = flag.String("proc-root", "/proc", "proc filesystem root")
		passwdFile      = flag.String("passwd-file", "/etc/passwd", "passwd-format UID to account mapping")
		poll            = flag.Duration("poll", time.Second, "connection sampling interval")
		metricsInterval = flag.Duration("metrics-interval", 5*time.Second, "system/process sampling interval")
		maxLogMB        = flag.Int64("max-log-mb", 32, "rotate event log at this size")
		keepLogs        = flag.Int("keep-logs", 4, "rotated event logs to retain")
	)
	flag.Parse()
	if !isLoopbackBind(*listen) {
		log.Fatal("refusing non-loopback HTTP bind; v1 UI/API is loopback-only")
	}
	if *poll < 100*time.Millisecond {
		log.Fatal("poll interval must be at least 100ms")
	}
	if *metricsInterval < time.Second {
		log.Fatal("metrics interval must be at least 1s")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	statePath, err := filepath.Abs(*stateDir)
	if err != nil {
		log.Fatal(err)
	}
	owners := uidmap.Load(*passwdFile)
	journal, err := store.Open(statePath, *maxLogMB*1024*1024, *keepLogs)
	if err != nil {
		log.Fatal(err)
	}
	defer journal.Close()

	local := localAddresses()
	tr := tracker.New()
	uiState := server.NewState(5000)
	uiState.SetSource("proc-sampler")
	sysSampler := metrics.NewSampler(*procRoot)
	if sample, sampleErr := sysSampler.Sample(time.Now().UTC()); sampleErr != nil {
		log.Printf("initial system sample warning: %v", sampleErr)
		uiState.AddSystem(sample, metrics.Diagnose(sample))
	} else {
		uiState.AddSystem(sample, metrics.Diagnose(sample))
	}

	snap, err := procnet.Read(*procRoot)
	if err != nil {
		log.Printf("initial proc scan warning: %v", err)
	}
	cache := make(map[uint64]model.ProcessInfo)
	initial := decorate(snap, *procRoot, owners, local, cache)
	tr.Baseline(initial)
	uiState.SetCurrent(initial)

	httpServer := &http.Server{
		Addr: *listen, Handler: (&server.Server{State: uiState}).Handler(),
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
	}
	go func() {
		log.Printf("Tattler UI/API listening on http://%s", *listen)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped: %v", err)
			cancel()
		}
	}()

	connectionTicker := time.NewTicker(*poll)
	defer connectionTicker.Stop()
	metricsTicker := time.NewTicker(*metricsInterval)
	defer metricsTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			_ = httpServer.Shutdown(shutdownCtx)
			return
		case <-connectionTicker.C:
			snap, err := procnet.Read(*procRoot)
			if err != nil {
				log.Printf("proc scan warning: %v", err)
			}
			current := decorate(snap, *procRoot, owners, local, cache)
			opened, closed := tr.Diff(current)
			now := time.Now().UTC()
			events := make([]model.Event, 0, len(opened)+len(closed))
			for _, c := range opened {
				events = append(events, model.NewEvent(now, hostname, "open", "proc-sampler", c))
			}
			for _, c := range closed {
				events = append(events, model.NewEvent(now, hostname, "close", "proc-sampler", c))
			}
			if err := journal.Append(events); err != nil {
				log.Printf("journal append: %v", err)
			}
			uiState.Add(events...)
			uiState.SetCurrent(current)
			pruneCache(cache, current)
		case now := <-metricsTicker.C:
			systemSample, systemErr := sysSampler.Sample(now.UTC())
			if systemErr != nil {
				log.Printf("system sample warning: %v", systemErr)
			}
			uiState.AddSystem(systemSample, metrics.Diagnose(systemSample))
		}
	}
}

func decorate(snap procnet.Snapshot, procRoot string, owners map[uint32]string, local map[netip.Addr]struct{}, cache map[uint64]model.ProcessInfo) []model.Connection {
	wanted := make(map[uint64]struct{})
	for _, c := range snap.Connections {
		if c.Inode != 0 {
			if _, ok := cache[c.Inode]; !ok {
				wanted[c.Inode] = struct{}{}
			}
		}
	}
	for inode, p := range procmap.Resolve(procRoot, wanted) {
		cache[inode] = p
	}
	out := make([]model.Connection, 0, len(snap.Connections))
	for _, c := range snap.Connections {
		c.Direction = tracker.Classify(c, snap.Listeners, local)
		if owner, ok := owners[c.UID]; ok {
			c.Owner = owner
		}
		if p, ok := cache[c.Inode]; ok {
			c.Process = p
		}
		out = append(out, c)
	}
	return out
}

func pruneCache(cache map[uint64]model.ProcessInfo, current []model.Connection) {
	live := make(map[uint64]struct{}, len(current))
	for _, c := range current {
		if c.Inode != 0 {
			live[c.Inode] = struct{}{}
		}
	}
	for inode := range cache {
		if _, ok := live[inode]; !ok {
			delete(cache, inode)
		}
	}
}

func localAddresses() map[netip.Addr]struct{} {
	out := make(map[netip.Addr]struct{})
	ifs, _ := net.Interfaces()
	for _, intf := range ifs {
		addrs, _ := intf.Addrs()
		for _, raw := range addrs {
			host := raw.String()
			if i := strings.LastIndex(host, "/"); i >= 0 {
				host = host[:i]
			}
			if a, err := netip.ParseAddr(host); err == nil {
				out[a.Unmap()] = struct{}{}
			}
		}
	}
	out[netip.MustParseAddr("127.0.0.1")] = struct{}{}
	out[netip.MustParseAddr("::1")] = struct{}{}
	return out
}

func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
