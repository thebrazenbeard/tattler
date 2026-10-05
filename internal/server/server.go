package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/thebrazenbeard/tattler/internal/metrics"
	"github.com/thebrazenbeard/tattler/internal/model"
)

type State struct {
	mu        sync.RWMutex
	current   []model.Connection
	recent    []model.Event
	samples   []metrics.Sample
	findings  []metrics.Finding
	max       int
	sampleMax int
	started   time.Time
	source    string
}

func NewState(max int) *State {
	return &State{max: max, sampleMax: 900, started: time.Now().UTC()}
}

func (s *State) SetSource(source string) {
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
}

func (s *State) SetCurrent(v []model.Connection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = append(s.current[:0], v...)
}

func (s *State) Add(events ...model.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, events...)
	if len(s.recent) > s.max {
		s.recent = append([]model.Event(nil), s.recent[len(s.recent)-s.max:]...)
	}
}

func (s *State) AddSystem(sample metrics.Sample, findings []metrics.Finding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, sample)
	if len(s.samples) > s.sampleMax {
		s.samples = append([]metrics.Sample(nil), s.samples[len(s.samples)-s.sampleMax:]...)
	}
	s.findings = append([]metrics.Finding(nil), findings...)
}

type Server struct{ State *State }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/status", s.status)
	mux.HandleFunc("/api/v1/current", s.current)
	mux.HandleFunc("/api/v1/events", s.events)
	mux.HandleFunc("/api/v1/system", s.system)
	mux.HandleFunc("/api/v1/findings", s.findings)
	mux.HandleFunc("/", s.index)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	var latest *metrics.Sample
	if len(s.State.samples) > 0 {
		copy := s.State.samples[len(s.State.samples)-1]
		latest = &copy
	}
	writeJSON(w, map[string]any{
		"schema_version":      1,
		"started_at":          s.State.started,
		"uptime_seconds":      int(time.Since(s.State.started).Seconds()),
		"collector":           s.State.source,
		"current_connections": len(s.State.current),
		"recent_events":       len(s.State.recent),
		"system_samples":      len(s.State.samples),
		"active_findings":     len(s.State.findings),
		"latest_system":       latest,
	})
}

func (s *Server) current(w http.ResponseWriter, _ *http.Request) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	writeJSON(w, s.State.current)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	limit := boundedLimit(r, 200, 5000)
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	start := len(s.State.recent) - limit
	if start < 0 {
		start = 0
	}
	writeJSON(w, s.State.recent[start:])
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	limit := boundedLimit(r, 120, 900)
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	start := len(s.State.samples) - limit
	if start < 0 {
		start = 0
	}
	writeJSON(w, s.State.samples[start:])
}

func (s *Server) findings(w http.ResponseWriter, _ *http.Request) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()
	writeJSON(w, s.State.findings)
}

func boundedLimit(r *http.Request, fallback, ceiling int) int {
	limit := fallback
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= ceiling {
			limit = n
		}
	}
	return limit
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

var indexHTML = []byte(`<!doctype html>
<html><head><meta charset="utf-8"><title>Tattler</title>
<style>
body{font:14px system-ui;background:#111;color:#eee;margin:24px}
.cards{display:flex;gap:12px;flex-wrap:wrap}.card{background:#1d1d1d;padding:12px 16px;border-radius:8px;min-width:130px}
.big{font-size:22px;font-weight:700}table{border-collapse:collapse;width:100%;margin-top:16px}
th,td{padding:7px;border-bottom:1px solid #333;text-align:left}code{color:#9fe}
.inbound{color:#ffb86c}.outbound{color:#8be9fd}.finding{background:#332b1c;padding:9px;margin:8px 0;border-radius:6px}
</style></head><body><h1>Tattler</h1><p id="meta">Loading...</p>
<div class="cards"><div class="card"><div>CPU</div><div class="big" id="cpu">-</div></div>
<div class="card"><div>I/O wait</div><div class="big" id="iow">-</div></div>
<div class="card"><div>Memory available</div><div class="big" id="mem">-</div></div>
<div class="card"><div>Load 1m</div><div class="big" id="load">-</div></div></div>
<h2>Findings</h2><div id="findings">None</div>
<h2>Current connections</h2><table><thead><tr><th>Direction</th><th>Process</th><th>Protocol</th><th>Local</th><th>Remote</th><th>State</th></tr></thead><tbody id="connections"></tbody></table>
<script>
function pct(n){return Number(n||0).toFixed(1)+'%'}
function esc(v){return String(v??'').replace(/[&<>\"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','\"':'&quot;',"'":'&#39;'}[c]))}
async function tick(){
 const st=await fetch('/api/v1/status').then(r=>r.json());
 const cs=await fetch('/api/v1/current').then(r=>r.json());
 const fs=await fetch('/api/v1/findings').then(r=>r.json());
 const s=st.latest_system||{};
 document.getElementById('meta').textContent=st.collector+' | '+st.current_connections+' connections | '+st.recent_events+' connection events';
 document.getElementById('cpu').textContent=pct(s.cpu_percent);
 document.getElementById('iow').textContent=pct(s.io_wait_percent);
 document.getElementById('load').textContent=Number(s.load1||0).toFixed(2);
 document.getElementById('mem').textContent=s.mem_total_kb?((s.mem_available_kb/s.mem_total_kb)*100).toFixed(1)+'%':'-';
 document.getElementById('findings').innerHTML=fs.length?fs.map(f=>'<div class="finding"><b>'+esc(f.severity).toUpperCase()+': '+esc(f.summary)+'</b><br>'+esc(f.evidence)+'</div>').join(''):'None';
 document.getElementById('connections').innerHTML=cs.map(c=>'<tr><td class="'+esc(c.direction)+'">'+esc(c.direction)+'</td><td>'+esc((c.process&&c.process.name)||c.owner||'?')+((c.process&&c.process.pid)?' ('+esc(c.process.pid)+')':'')+'</td><td>'+esc(c.protocol)+'</td><td><code>'+esc(c.local)+'</code></td><td><code>'+esc(c.remote)+'</code></td><td>'+esc(c.state)+'</td></tr>').join('');
}
tick();setInterval(tick,2000)
</script></body></html>`)
