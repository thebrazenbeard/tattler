package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type managedProcess interface {
	PID() int
	Kill() error
}

type osManagedProcess struct {
	process *os.Process
}

func (p *osManagedProcess) PID() int { return p.process.Pid }
func (p *osManagedProcess) Kill() error { return p.process.Kill() }

type AgentState struct {
	Running bool   `json:"running"`
	Managed bool   `json:"managed"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DesktopSnapshot struct {
	Agent    AgentState       `json:"agent"`
	Status   map[string]any   `json:"status"`
	Current  []map[string]any `json:"current"`
	Findings []map[string]any `json:"findings"`
	Events   []map[string]any `json:"events"`
}

type launcher func(string, ...string) (managedProcess, error)

type App struct {
	mu         sync.Mutex
	baseURL    string
	client     *http.Client
	health     func() bool
	launch     launcher
	executable func() (string, error)
	cacheDir   func() (string, error)
	child      managedProcess
}

func NewApp() *App {
	app := &App{
		baseURL:    "http://127.0.0.1:9147",
		client:     &http.Client{Timeout: 1500 * time.Millisecond},
		executable: os.Executable,
		cacheDir:   os.UserCacheDir,
	}
	app.health = app.probeHealth
	app.launch = func(path string, args ...string) (managedProcess, error) {
		cmd := exec.Command(path, args...)
		if err := cmd.Start(); err != nil { return nil, err }
		return &osManagedProcess{process: cmd.Process}, nil
	}
	return app
}

func newApp(baseURL string, health func() bool, launch launcher, executable func() (string, error), cacheDir func() (string, error)) *App {
	app := &App{baseURL: baseURL, client: &http.Client{Timeout: 500 * time.Millisecond}, health: health, launch: launch, executable: executable, cacheDir: cacheDir}
	if app.health == nil { app.health = app.probeHealth }
	return app
}

func (a *App) Startup(context.Context) { _ = a.StartAgent() }

func (a *App) Shutdown(context.Context) {
	a.mu.Lock()
	child := a.child
	a.child = nil
	a.mu.Unlock()
	if child != nil { _ = child.Kill() }
}

func (a *App) AgentState() AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.health() {
		if a.child != nil { return AgentState{Running: true, Managed: true, PID: a.child.PID()} }
		return AgentState{Running: true}
	}
	if a.child != nil { return AgentState{Managed: true, PID: a.child.PID(), Error: "agent not ready"} }
	return AgentState{}
}

func (a *App) StartAgent() AgentState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.health() {
		if a.child != nil { return AgentState{Running: true, Managed: true, PID: a.child.PID()} }
		return AgentState{Running: true}
	}
	if a.child != nil { return AgentState{Managed: true, PID: a.child.PID(), Error: "agent not ready"} }

	exe, err := a.executable()
	if err != nil { return AgentState{Error: err.Error()} }
	cache, err := a.cacheDir()
	if err != nil { return AgentState{Error: err.Error()} }
	stateDir := filepath.Join(cache, "Tattler", "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil { return AgentState{Error: err.Error()} }

	candidates := []string{
		filepath.Join(filepath.Dir(exe), "tattler-windows-amd64.exe"),
		filepath.Join(filepath.Dir(exe), "tattler.exe"),
	}
	var child managedProcess
	for _, candidate := range candidates {
		child, err = a.launch(candidate, "--listen", "127.0.0.1:9147", "--state-dir", stateDir)
		if err == nil { break }
	}
	if err != nil { return AgentState{Error: err.Error()} }
	a.child = child
	for i := 0; i < 30; i++ {
		if a.health() { return AgentState{Running: true, Managed: true, PID: child.PID()} }
		time.Sleep(100 * time.Millisecond)
	}
	_ = child.Kill()
	a.child = nil
	return AgentState{Error: "agent did not become ready"}
}

func (a *App) Snapshot() (DesktopSnapshot, error) {
	var out DesktopSnapshot
	out.Agent = a.AgentState()
	if !out.Agent.Running { return out, errors.New("Tattler agent is offline") }
	if err := a.getJSON("/api/v1/status", &out.Status); err != nil { return out, err }
	if err := a.getJSON("/api/v1/current", &out.Current); err != nil { return out, err }
	if err := a.getJSON("/api/v1/findings", &out.Findings); err != nil { return out, err }
	if err := a.getJSON("/api/v1/events?limit=200", &out.Events); err != nil { return out, err }
	return out, nil
}

func (a *App) probeHealth() bool {
	req, err := http.NewRequest(http.MethodGet, a.baseURL+"/healthz", nil)
	if err != nil { return false }
	resp, err := a.client.Do(req)
	if err != nil { return false }
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (a *App) getJSON(path string, target any) error {
	req, err := http.NewRequest(http.MethodGet, a.baseURL+path, nil)
	if err != nil { return err }
	resp, err := a.client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return errors.New(resp.Status) }
	return json.NewDecoder(resp.Body).Decode(target)
}
