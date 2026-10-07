package server

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/thebrazenbeard/tattler/internal/metrics"
	"github.com/thebrazenbeard/tattler/internal/model"
)

type State struct {
	mu                  sync.RWMutex
	current             []model.Connection
	recent              []model.Event
	samples             []metrics.Sample
	findings            []metrics.Finding
	semantic            []model.SemanticObservation
	max                 int
	sampleMax           int
	semanticMax         int
	started             time.Time
	source              string
	networkScanState    string
	lastScanAttempt     time.Time
	lastCompleteScan    time.Time
	partialScansSkipped uint64
}

func NewState(max int) *State {
	return &State{
		max: max, sampleMax: 900, semanticMax: max, started: time.Now().UTC(),
		semantic: make([]model.SemanticObservation, 0), networkScanState: "pending",
	}
}

func (s *State) SetSource(source string) {
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
}

// NoteNetworkScan records attempted collection without exposing raw error paths.
// A partial scan leaves current evidence unchanged until a full pass succeeds.
func (s *State) NoteNetworkScan(at time.Time, complete bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastScanAttempt = at.UTC()
	if complete {
		s.networkScanState = "complete"
		s.lastCompleteScan = at.UTC()
	} else {
		s.networkScanState = "incomplete"
		s.partialScansSkipped++
	}
}

func nullableScanTime(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return at
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

func (s *State) AddSemantic(events ...model.SemanticObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.semantic = append(s.semantic, events...)
	if len(s.semantic) > s.semanticMax {
		s.semantic = append([]model.SemanticObservation(nil), s.semantic[len(s.semantic)-s.semanticMax:]...)
	}
}

//go:embed tattler-icon.png
var tattlerIcon []byte

type Server struct{ State *State }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/status", s.status)
	mux.HandleFunc("/api/v1/current", s.current)
	mux.HandleFunc("/api/v1/events", s.events)
	mux.HandleFunc("/api/v1/system", s.system)
	mux.HandleFunc("/api/v1/findings", s.findings)
	mux.HandleFunc("/api/v1/semantic-events", s.semanticEvents)
	mux.HandleFunc("/assets/tattler-icon.png", s.icon)
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
		"schema_version":       1,
		"started_at":           s.State.started,
		"uptime_seconds":       int(time.Since(s.State.started).Seconds()),
		"collector":            s.State.source,
		"current_observations": len(s.State.current),
		"current_connections":  len(s.State.current),
		"recent_events":        len(s.State.recent),
		"system_samples":       len(s.State.samples),
		"active_findings":      len(s.State.findings),
		"semantic_events":      len(s.State.semantic),
		"latest_system":        latest,
		"network_scan": map[string]any{
			"state":                    s.State.networkScanState,
			"last_attempt_at":          nullableScanTime(s.State.lastScanAttempt),
			"last_complete_at":         nullableScanTime(s.State.lastCompleteScan),
			"incomplete_scans_skipped": s.State.partialScansSkipped,
		},
	})
}

type currentObservation struct {
	model.Connection
	FirstSeen           *time.Time `json:"first_seen,omitempty"`
	AgeSeconds          int64      `json:"age_seconds"`
	TrackingKey         string     `json:"tracking_key"`
	IdentityBasis       string     `json:"identity_basis"`
	SameTrackingKeyRows int        `json:"same_tracking_key_rows"`
}

func (s *Server) current(w http.ResponseWriter, _ *http.Request) {
	s.State.mu.RLock()
	defer s.State.mu.RUnlock()

	now := time.Now().UTC()
	out := make([]currentObservation, 0, len(s.State.current))
	counts := make(map[string]int, len(s.State.current))
	for _, c := range s.State.current {
		counts[c.Key()]++
	}
	for _, c := range s.State.current {
		identity := c.Key()
		sum := sha256.Sum256([]byte(identity))
		basis := "endpoint_only"
		if c.Inode != 0 {
			basis = "inode_endpoint"
		} else if c.Process.PID != 0 {
			basis = "pid_endpoint_fallback"
		}
		item := currentObservation{Connection: c, TrackingKey: hex.EncodeToString(sum[:]), IdentityBasis: basis, SameTrackingKeyRows: counts[identity]}
		if !c.FirstSeen.IsZero() {
			first := c.FirstSeen.UTC()
			item.FirstSeen = &first
			item.AgeSeconds = int64(now.Sub(first).Seconds())
			if item.AgeSeconds < 0 {
				item.AgeSeconds = 0
			}
		}
		out = append(out, item)
	}
	writeJSON(w, out)
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

func (s *Server) semanticEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := boundedLimit(r, 200, 5000)
		s.State.mu.RLock()
		defer s.State.mu.RUnlock()
		start := len(s.State.semantic) - limit
		if start < 0 {
			start = 0
		}
		writeJSON(w, s.State.semantic[start:])
	case http.MethodPost:
		contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
		if contentType != "application/json" {
			http.Error(w, "semantic events require application/json", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var in model.SemanticObservation
		if err := dec.Decode(&in); err != nil {
			http.Error(w, "invalid semantic event: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "invalid semantic event: trailing JSON", http.StatusBadRequest)
			return
		}
		normalizeSemantic(&in)
		if msg := validateSemantic(in); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		event := model.NewSemanticObservation(time.Now().UTC(), in)
		s.State.AddSemantic(event)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(event)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func normalizeSemantic(in *model.SemanticObservation) {
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	in.Protocol = normalizeReportedProtocol(in.Protocol)
	in.Direction = strings.ToLower(strings.TrimSpace(in.Direction))
	in.Peer = strings.TrimSpace(in.Peer)
	in.Method = strings.ToUpper(strings.TrimSpace(in.Method))
	in.Route = strings.TrimSpace(in.Route)
	in.Reporter = strings.TrimSpace(in.Reporter)
	in.Provider = strings.TrimSpace(in.Provider)
	in.EventType = strings.TrimSpace(in.EventType)
	in.DeliveryID = strings.TrimSpace(in.DeliveryID)
	in.ErrorType = strings.ToLower(strings.TrimSpace(in.ErrorType))
}

func normalizeReportedProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "http/1", "http/1.0", "http/1.1", "http1":
		return "http1"
	case "http/2", "h2", "http2":
		return "http2"
	case "http/3", "h3", "http3":
		return "http3"
	case "ws", "websocket":
		return "websocket"
	case "wss", "websocket_tls":
		return "websocket_tls"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func validateSemantic(in model.SemanticObservation) string {
	switch in.Kind {
	case model.SemanticHTTPTransaction, model.SemanticWebhookDelivery, model.SemanticWebSocketSession, model.SemanticGRPCRPC:
	default:
		return "unsupported semantic event kind"
	}
	switch in.Protocol {
	case "http", "https", "http1", "http2", "http3", "websocket", "websocket_tls", "grpc", "grpcs":
	default:
		return "unsupported reported protocol"
	}
	if in.Direction != "" && in.Direction != "inbound" && in.Direction != "outbound" {
		return "direction must be inbound or outbound"
	}
	if len(in.Peer) > 253 || strings.ContainsAny(in.Peer, "/?#") {
		return "peer must be a host[:port], not a URL"
	}
	if len(in.Method) > 16 || strings.IndexFunc(in.Method, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return "method is invalid"
	}
	if len(in.Route) > 256 || strings.Contains(in.Route, "?") || strings.Contains(in.Route, "#") || strings.Contains(in.Route, "://") {
		return "route must be a low-cardinality route/template without query, fragment, or URL scheme"
	}
	if in.Status != 0 && (in.Status < 100 || in.Status > 599) {
		return "status must be an HTTP status code"
	}
	if in.DurationMS < 0 || in.BytesIn < 0 || in.BytesOut < 0 || in.Retry < 0 {
		return "duration, byte counts, and retry must be non-negative"
	}
	if len(in.Reporter) > 128 || len(in.Provider) > 128 || len(in.EventType) > 128 || len(in.DeliveryID) > 256 || len(in.ErrorType) > 128 {
		return "semantic metadata exceeds size limits"
	}
	if strings.IndexFunc(in.ErrorType, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return "error_type must be a low-cardinality token"
	}
	if in.Kind == model.SemanticWebhookDelivery && in.Method == "" {
		return "webhook_delivery requires method"
	}
	return ""
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

func (s *Server) icon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(tattlerIcon)
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
<html><head><meta charset="utf-8"><title>Tattler</title><link rel="icon" href="/assets/tattler-icon.png">
<style>
body{font:14px system-ui;background:#111;color:#eee;margin:24px}
.cards{display:flex;gap:12px;flex-wrap:wrap}.card{background:#1d1d1d;padding:12px 16px;border-radius:8px;min-width:130px}
.big{font-size:22px;font-weight:700}table{border-collapse:collapse;width:100%;margin-top:16px}
th,td{padding:7px;border-bottom:1px solid #333;text-align:left}code{color:#9fe}
.brand{display:flex;align-items:center;gap:12px}.brand-icon{width:56px;height:56px;border-radius:12px}.brand h1{margin:0}
.inbound{color:#ffb86c}.outbound{color:#8be9fd}.finding{background:#332b1c;padding:9px;margin:8px 0;border-radius:6px}
</style></head><body><div class="brand"><img class="brand-icon" src="/assets/tattler-icon.png" alt="Tattler icon"><h1>Tattler</h1></div><p id="meta">Loading...</p>
<div class="cards"><div class="card"><div>CPU</div><div class="big" id="cpu">-</div></div>
<div class="card"><div>I/O wait</div><div class="big" id="iow">-</div></div>
<div class="card"><div>Memory available</div><div class="big" id="mem">-</div></div>
<div class="card"><div>Load 1m</div><div class="big" id="load">-</div></div></div>
<h2>Findings</h2><div id="findings">None</div>
<h2>Network activity</h2><table><thead><tr><th>Kind</th><th>Direction</th><th>Process</th><th>Transport</th><th>Protocol evidence</th><th>Local</th><th>Remote</th><th title="Continuous time Tattler has observed this live endpoint">Observed for ↓</th><th>State</th></tr></thead><tbody id="connections"></tbody></table>
<h2>Semantic activity</h2><table><thead><tr><th>Observed</th><th>Kind</th><th>Protocol</th><th>Direction</th><th>Reporter</th><th>Peer</th><th>Route</th><th>Status</th><th>Duration</th><th>Detail</th></tr></thead><tbody id="semantic"></tbody></table>
<script>
function pct(n){return Number(n||0).toFixed(1)+'%'}
function unavailable(s,n){return Array.isArray(s.unavailable_metrics)&&s.unavailable_metrics.includes(n)}
function esc(v){return String(v??'').replace(/[&<>\"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','\"':'&quot;',"'":'&#39;'}[c]))}
function ageLabel(seconds){const n=Math.max(0,Number(seconds||0));if(n<60)return Math.floor(n)+'s';if(n<3600)return Math.floor(n/60)+'m '+Math.floor(n%60)+'s';if(n<86400)return Math.floor(n/3600)+'h '+Math.floor((n%3600)/60)+'m';return Math.floor(n/86400)+'d '+Math.floor((n%86400)/3600)+'h'}
async function tick(){
 const st=await fetch('/api/v1/status').then(r=>r.json());
 const cs=await fetch('/api/v1/current').then(r=>r.json());
 const fs=await fetch('/api/v1/findings').then(r=>r.json());
 const sem=await fetch('/api/v1/semantic-events?limit=200').then(r=>r.json());
 const s=st.latest_system||{};
 document.getElementById('meta').textContent=(s.platform?s.platform+' | ':'')+st.collector+' | '+(st.current_observations??st.current_connections)+' observations | '+st.recent_events+' observation events';
 document.getElementById('cpu').textContent=pct(s.cpu_percent);
 document.getElementById('iow').textContent=unavailable(s,'io_wait_percent')?'n/a':pct(s.io_wait_percent);
 document.getElementById('load').textContent=unavailable(s,'load_average')?'n/a':Number(s.load1||0).toFixed(2);
 document.getElementById('mem').textContent=s.mem_total_kb?((s.mem_available_kb/s.mem_total_kb)*100).toFixed(1)+'%':'-';
 document.getElementById('findings').innerHTML=fs.length?fs.map(f=>'<div class="finding"><b>'+esc(f.severity).toUpperCase()+': '+esc(f.summary)+'</b><br>'+esc(f.evidence)+'</div>').join(''):'None';
 const live=[...cs].sort((a,b)=>Number(b.age_seconds||0)-Number(a.age_seconds||0)||String(a.local||'').localeCompare(String(b.local||''))||String(a.remote||'').localeCompare(String(b.remote||''))||String((a.process||{}).pid||'').localeCompare(String((b.process||{}).pid||'')));
 document.getElementById('connections').innerHTML=live.map(c=>'<tr><td>'+esc(c.kind||'unknown')+'</td><td class="'+esc(c.direction)+'">'+esc(c.direction||'-')+'</td><td>'+esc((c.process&&c.process.name)||c.owner||'?')+((c.process&&c.process.pid)?' ('+esc(c.process.pid)+')':'')+'</td><td>'+esc(c.protocol)+'</td><td>'+esc(protocolEvidence(c))+'</td><td><code>'+esc(c.local)+'</code></td><td><code>'+esc((c.remote&&c.remote!=='invalid AddrPort'&&c.remote!=='0.0.0.0:0'&&c.remote!=='[::]:0')?c.remote:'-')+'</code></td><td>'+esc(ageLabel(c.age_seconds))+'</td><td>'+esc(c.state||'-')+'</td></tr>').join('');
 document.getElementById('semantic').innerHTML=sem.slice().reverse().map(e=>'<tr><td>'+esc(e.observed_at||'')+'</td><td>'+esc(e.kind||'')+'</td><td>'+esc(e.protocol||'')+'</td><td>'+esc(e.direction||'-')+'</td><td>'+esc(e.reporter||'-')+'</td><td>'+esc(e.peer||'-')+'</td><td><code>'+esc(e.route||'-')+'</code></td><td>'+esc(e.status||'-')+'</td><td>'+esc(e.duration_ms?e.duration_ms+' ms':'-')+'</td><td>'+esc(semanticDetail(e))+'</td></tr>').join('');
}
tick();setInterval(tick,2000)
</script></body></html>`)
