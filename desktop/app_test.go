package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeProcess struct {
	pid    int
	killed bool
}

func (p *fakeProcess) PID() int    { return p.pid }
func (p *fakeProcess) Kill() error { p.killed = true; return nil }

func TestStartAgentAttachesWithoutLaunch(t *testing.T) {
	launches := 0
	app := newApp("http://127.0.0.1:9147", func() bool { return true }, func(string, ...string) (managedProcess, error) { launches++; return &fakeProcess{pid: 10}, nil }, func() (string, error) { return `C:\\Tools\\tattler-companion.exe`, nil }, func() (string, error) { return t.TempDir(), nil })
	state := app.StartAgent()
	if !state.Running || state.Managed {
		t.Fatalf("state=%+v, want running attached agent", state)
	}
	if launches != 0 {
		t.Fatalf("launches=%d, want 0", launches)
	}
}

func TestStartAgentLaunchesOnlyOwnedChild(t *testing.T) {
	ready := false
	proc := &fakeProcess{pid: 42}
	cache := t.TempDir()
	var launchedPath string
	var launchedArgs []string
	app := newApp("http://127.0.0.1:9147", func() bool { return ready }, func(path string, args ...string) (managedProcess, error) {
		launchedPath = path
		launchedArgs = append([]string(nil), args...)
		ready = true
		return proc, nil
	}, func() (string, error) { return `C:\\Tools\\tattler-companion.exe`, nil }, func() (string, error) { return cache, nil })
	state := app.StartAgent()
	if !state.Running || !state.Managed || state.PID != 42 {
		t.Fatalf("state=%+v, want managed running pid 42", state)
	}
	if filepath.Base(launchedPath) != "tattler-windows-amd64.exe" {
		t.Fatalf("launched %q", launchedPath)
	}
	joined := strings.Join(launchedArgs, " ")
	if !strings.Contains(joined, "--listen 127.0.0.1:9147") || !strings.Contains(joined, "--state-dir "+filepath.Join(cache, "Tattler", "state")) {
		t.Fatalf("args=%q", joined)
	}
	app.Shutdown(context.Background())
	if !proc.killed {
		t.Fatal("owned child was not stopped")
	}
}

func TestStartAgentReportsLaunchFailure(t *testing.T) {
	app := newApp("http://127.0.0.1:9147", func() bool { return false }, func(string, ...string) (managedProcess, error) { return nil, errors.New("missing agent") }, func() (string, error) { return `C:\\Tools\\tattler-companion.exe`, nil }, func() (string, error) { return t.TempDir(), nil })
	state := app.StartAgent()
	if state.Running || state.Managed || state.Error == "" {
		t.Fatalf("state=%+v, want explicit offline error", state)
	}
}

func TestShutdownDoesNotKillAttachedAgent(t *testing.T) {
	app := newApp("http://127.0.0.1:9147", func() bool { return true }, func(string, ...string) (managedProcess, error) {
		t.Fatal("launch should not be called")
		return nil, nil
	}, func() (string, error) { return `C:\\Tools\\tattler-companion.exe`, nil }, func() (string, error) { return t.TempDir(), nil })
	app.StartAgent()
	app.Shutdown(context.Background())
}

func TestSnapshotReadsLoopbackAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"collector":"windows-ip-helper","current_observations":1}`))
	})
	mux.HandleFunc("/api/v1/current", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"kind":"udp_endpoint","protocol":"udp","local":"127.0.0.1:5353","remote":"0.0.0.0:0"}]`))
	})
	mux.HandleFunc("/api/v1/findings", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	server := httptest.NewServer(mux)
	defer server.Close()
	app := newApp(server.URL, nil, func(string, ...string) (managedProcess, error) { return nil, nil }, func() (string, error) { return `C:\\Tools\\tattler-companion.exe`, nil }, func() (string, error) { return t.TempDir(), nil })
	snapshot, err := app.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Status["collector"]; got != "windows-ip-helper" {
		t.Fatalf("collector=%v", got)
	}
	if len(snapshot.Current) != 1 || snapshot.Current[0]["kind"] != "udp_endpoint" {
		t.Fatalf("current=%v", snapshot.Current)
	}
}

func TestStartAgentMissingSiblingReportsActionableError(t *testing.T) {
	app := newApp("http://127.0.0.1:9147", func() bool { return false }, func(path string, _ ...string) (managedProcess, error) {
		return nil, &os.PathError{Op: "fork/exec", Path: path, Err: os.ErrNotExist}
	}, func() (string, error) { return `C:\Tools\tattler-desktop-windows-amd64.exe`, nil }, func() (string, error) { return t.TempDir(), nil })

	state := app.StartAgent()
	if state.Running || state.Managed {
		t.Fatalf("state=%+v, want offline", state)
	}
	if !strings.Contains(state.Error, "Tattler agent executable not found") {
		t.Fatalf("error=%q, want actionable missing-agent message", state.Error)
	}
	if !strings.Contains(state.Error, "tattler-windows-amd64.exe") {
		t.Fatalf("error=%q, want expected filename", state.Error)
	}
}

func TestStartAgentRestartsUnhealthyManagedChild(t *testing.T) {
	ready := false
	stale := &fakeProcess{pid: 41}
	fresh := &fakeProcess{pid: 42}
	launches := 0

	app := newApp("http://127.0.0.1:9147", func() bool { return ready }, func(string, ...string) (managedProcess, error) {
		launches++
		ready = true
		return fresh, nil
	}, func() (string, error) {
		return `C:\Tools\tattler-desktop-windows-amd64.exe`, nil
	}, func() (string, error) {
		return t.TempDir(), nil
	})
	app.child = stale

	state := app.StartAgent()

	if !stale.killed {
		t.Fatal("stale managed child was not cleared before restart")
	}
	if launches != 1 {
		t.Fatalf("launches=%d, want 1", launches)
	}
	if !state.Running || !state.Managed || state.PID != 42 {
		t.Fatalf("state=%+v, want fresh managed running pid 42", state)
	}
}

func TestSectionReadsAreIndependentAndOptionalSemantic(t *testing.T) {
	calls := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { calls["health"]++; _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		calls["status"]++
		_, _ = w.Write([]byte(`{"collector":"test"}`))
	})
	mux.HandleFunc("/api/v1/current", func(w http.ResponseWriter, _ *http.Request) {
		calls["current"]++
		_, _ = w.Write([]byte(`[{"kind":"tcp_listener"}]`))
	})
	mux.HandleFunc("/api/v1/findings", func(w http.ResponseWriter, _ *http.Request) { calls["findings"]++; _, _ = w.Write([]byte(`[]`)) })
	mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, _ *http.Request) { calls["events"]++; _, _ = w.Write([]byte(`[]`)) })
	mux.HandleFunc("/api/v1/semantic-events", func(w http.ResponseWriter, _ *http.Request) { calls["semantic"]++; w.WriteHeader(http.StatusNotFound) })
	server := httptest.NewServer(mux)
	defer server.Close()
	app := newApp(server.URL, nil, func(string, ...string) (managedProcess, error) { return nil, nil },
		func() (string, error) { return "test.exe", nil }, func() (string, error) { return t.TempDir(), nil })
	fast, err := app.Network()
	if err != nil || len(fast.Current) != 1 {
		t.Fatalf("fast=%v err=%v", fast, err)
	}
	if calls["findings"] != 0 || calls["events"] != 0 || calls["semantic"] != 0 {
		t.Fatalf("fast read fetched slow sections: %v", calls)
	}
	if _, err = app.ReadFindings(); err != nil {
		t.Fatal(err)
	}
	if _, err = app.ReadEvents(); err != nil {
		t.Fatal(err)
	}
	sem, err := app.ReadSemantic()
	if err != nil || sem != nil {
		t.Fatalf("optional older semantic=%v err=%v", sem, err)
	}
	if calls["status"] != 1 || calls["current"] != 1 || calls["findings"] != 1 || calls["events"] != 1 || calls["semantic"] != 1 {
		t.Fatalf("unexpected calls=%v", calls)
	}
}
