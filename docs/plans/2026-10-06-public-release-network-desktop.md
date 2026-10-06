# Tattler Public Release Network + Desktop Implementation Plan

> **For agentic workers:** Use the host's available task-by-task implementation workflow. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn draft PR #2 into a source-qualified public-release candidate with honest TCP/UDP/listener telemetry, a Windows desktop companion, aligned public documentation/licensing, and green exact-head CI/provenance.

**Architecture:** Keep the existing root Go agent small and dependency-free. Extend its observation model and Linux/Windows collectors in place, then add a separate `desktop/` Wails v2.14.0 module that consumes the loopback API through a Go-bound frontend service. CI qualifies the agent, DSM package, and desktop companion as separate build/runtime subjects.

**Tech Stack:** Go root module; Linux `/proc`; Windows IP Helper APIs; Wails v2.14.0 + WebView2 for the Windows companion; static HTML/CSS/JS; Python SPK tooling; GitHub Actions.

## Global Constraints

- Preserve Tattler as observation-only: no firewall/routing/DNS mutation, payload capture, or remote telemetry.
- Public terminology is network activity / endpoint telemetry, not complete connection or packet capture.
- Observation kinds are `tcp_session`, `tcp_listener`, and `udp_endpoint`.
- Windows UDP uses `GetExtendedUdpTable(UDP_TABLE_OWNER_PID)`; do not infer remote peers that API does not provide.
- ETW, pcap, and eBPF are outside this release.
- Keep the root agent module free of third-party Go dependencies; Wails lives in `desktop/`.
- The desktop app may stop only an agent child process it launched itself.
- Preserve existing API fields where practical; additive compatibility is preferred.
- Do not weaken the deterministic DSM/package-source provenance check to obtain green CI.
- Keep source/build/package/runtime/release claims separate.
- Missing WebView2 handling recommendation for this plan: build the companion with Wails `-webview2 browser` so the app does not silently download/install a runtime.

---

### Task 1: Add typed TCP listener and UDP endpoint observations

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/procnet/procnet.go`
- Modify: `internal/procnet/procnet_test.go`
- Modify: `internal/tracker/tracker.go`
- Modify: `internal/tracker/tracker_test.go`
- Modify: `internal/wincollect/wincollect_windows.go`
- Modify: `internal/wincollect/wincollect_windows_test.go`
- Modify: `cmd/tattler/main.go`
- Modify: `cmd/tattler/main_test.go`
- Modify: `internal/server/server.go`
- Modify/Create focused server tests under `internal/server/`

**Interfaces:**
- Consumes: existing `model.Connection`, `procnet.Read`, `wincollect.Collector.Connections`, `tracker.Tracker`, and `/api/v1/current`.
- Produces: additive `Connection.Kind string` JSON field; kinds `tcp_session`, `tcp_listener`, `udp_endpoint`; status field `current_observations` while retaining `current_connections`.

- [ ] **Step 1: Add the focused failing tests**

Add Linux fixture coverage with:
- IPv4 TCP LISTEN row -> one `tcp_listener`;
- IPv4 established TCP row -> one `tcp_session`;
- IPv4 UDP unconnected row `00000000:port -> 00000000:0000` -> retained `udp_endpoint`;
- IPv6 UDP row -> retained `udp_endpoint`;
- UDP direction remains empty;
- listener direction is `listen`.

Add model/tracker tests proving:
- kind participates in the key;
- PID participates when inode is zero so same local UDP tuple owned by different Windows PIDs does not collapse;
- tracker still emits presence transitions deterministically.

Add Windows tests using live loopback fixtures:
- TCP listener appears as `tcp_listener`;
- established loopback client/server rows are `tcp_session`;
- a bound UDP4 socket owned by the test PID appears as `udp_endpoint`;
- when IPv6 UDP bind is available, the owned UDP6 endpoint appears with the test PID;
- no UDP observation receives an invented remote peer/direction.

Add server tests proving `current_observations` equals the current snapshot count and `current_connections` remains present as a compatibility alias.

- [ ] **Step 2: Verify the relevant failures**

Run on Linux-capable CI/local environment:
`go test ./internal/model ./internal/procnet ./internal/tracker ./internal/server`

Expected: new observation-kind/listener/UDP assertions fail because listeners/UDP endpoints are not yet exposed and the model has no `kind`.

Run on Windows:
`go test ./internal/wincollect ./cmd/tattler`

Expected: UDP/listener exposure assertions fail against the TCP-only Windows collector.

- [ ] **Step 3: Implement the minimum behavior**

In `model.Connection`, add `Kind string `json:"kind,omitempty"``. Define constants for the three public kinds. Update `Key()` to bind kind plus protocol/local/remote and ownership identity; preserve inode as the strongest Linux socket identity and include PID when inode is zero.

In `procnet.Read`:
- tag TCP LISTEN rows as `tcp_listener`;
- tag other TCP rows as `tcp_session`;
- tag every parsed UDP row as `udp_endpoint`;
- retain unconnected UDP rows instead of filtering them out;
- keep listener rows in `Snapshot.Listeners` for classification and expose them to the caller through the decorated current observation set.

In `decorate`:
- decorate TCP sessions, TCP listeners, and UDP endpoints with UID/owner/process evidence;
- classify only TCP sessions through `tracker.Classify`;
- set listener direction to `listen`;
- leave UDP direction empty.

In `wincollect`:
- stop dropping TCP LISTEN rows; emit them as `tcp_listener`;
- keep existing session classification for non-listening TCP rows;
- bind `GetExtendedUdpTable`;
- implement IPv4 and IPv6 OWNER_PID table parsing with correct structure alignment and network-byte-order port conversion;
- enrich UDP rows through the existing process lookup;
- represent no peer as the zero/unspecified endpoint from the relevant address family and leave direction/state empty;
- join per-table errors so successful protocol/family tables survive partial failure.

In the server/dashboard:
- rename visible UI wording to “Network activity” / “observations”;
- add `current_observations`;
- retain `current_connections` for compatibility;
- render `kind` explicitly and render an unspecified remote peer as `-`.

- [ ] **Step 4: Verify the focused pass**

Linux/root command:
`go test ./internal/model ./internal/procnet ./internal/tracker ./internal/server ./cmd/tattler`

Expected: PASS.

Windows command:
`go test ./internal/wincollect ./cmd/tattler ./internal/server`

Expected: PASS with owned TCP listener/session and UDP endpoint evidence.

- [ ] **Step 5: Run the affected integration check**

Run:
`go test ./... && go vet ./...`

Expected: PASS on both Ubuntu and Windows runners.

- [ ] **Step 6: Commit the passing deliverable**

Commit message:
`feat: broaden typed network endpoint telemetry`

---

### Task 2: Add the Windows desktop companion

**Files:**
- Create: `desktop/go.mod`
- Create: `desktop/go.sum`
- Create: `desktop/main.go`
- Create: `desktop/app.go`
- Create: `desktop/app_test.go`
- Create: `desktop/frontend/dist/index.html`
- Create: `desktop/wails.json`
- Create: `desktop/build/appicon.png` by reusing the repository-owned Tattler icon bytes during implementation/build preparation
- Modify: `.gitignore`

**Interfaces:**
- Consumes: local HTTP endpoints `/healthz`, `/api/v1/status`, `/api/v1/current`, `/api/v1/findings`, `/api/v1/events`.
- Produces: Wails-bound methods `Snapshot() (DesktopSnapshot, error)`, `StartAgent() AgentState`, and `AgentState() AgentState`; child ownership remains internal to the app.

- [ ] **Step 1: Add the focused failing tests**

Create `app_test.go` around an injected HTTP base URL and injectable process launcher.

Test:
- healthy existing agent -> app reports attached and does not launch;
- unavailable agent + sibling executable -> exactly one launch with loopback/default state-dir arguments;
- unavailable agent + missing sibling executable -> explicit offline/error state, no panic;
- app shutdown stops only the injected child it launched;
- attached external agent is never stopped;
- snapshot decodes status/current/findings/events and preserves observation kinds.

- [ ] **Step 2: Verify the relevant failure**

Run:
`cd desktop && go test ./...`

Expected: fail because the companion module/service does not exist.

- [ ] **Step 3: Implement the minimum behavior**

Pin `github.com/wailsapp/wails/v2 v2.14.0` in the desktop module.

Implement an `App` service with:
- loopback base URL fixed to `http://127.0.0.1:9147`;
- bounded HTTP client timeouts;
- startup health probe;
- sibling-agent search beside the companion executable for `tattler-windows-amd64.exe` then `tattler.exe`;
- state directory under `os.UserCacheDir()/Tattler/state`;
- child launch using `exec.Command` without shell interpolation;
- ownership flag/process handle retained only for the child it launches;
- shutdown cleanup only for that owned child.

Use static frontend assets and `window.go.main.App.*` Wails bindings. The UI shows agent state, host/system summary, findings, and typed network observations. It refreshes on a bounded interval and visibly reports API errors/offline state.

Configure the Wails app as a Windows desktop window with the repository icon, no public HTTP listener, and no independent telemetry collector.

- [ ] **Step 4: Verify the focused pass**

Run:
`cd desktop && go test ./...`

Expected: PASS.

- [ ] **Step 5: Run the affected integration check**

On Windows:
`go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0`
then:
`cd desktop && wails build -clean -platform windows/amd64 -trimpath -webview2 browser`

Expected: companion executable produced under the Wails build output directory.

With the root agent built beside it, start the companion test harness and verify its bound backend reports the live loopback status/current observations.

- [ ] **Step 6: Commit the passing deliverable**

Commit message:
`feat: add Windows desktop companion`

---

### Task 3: Close CI provenance and public-release hygiene

**Files:**
- Modify: `.github/workflows/build.yml`
- Modify: `package-source/release.json`
- Replace: exact `package-source/public/releases/Tattler-armada38x-<version>.spk` generated from the pinned CI toolchain
- Modify if required by version advance: `spk/INFO`
- Create: `LICENSE`
- Create: `COMMERCIAL_LICENSE.md`
- Create: `NOTICE`
- Create: `CONTRIBUTING.md`
- Create: `CLA.md`
- Create: `SECURITY.md`
- Create: `tools/release_hygiene.py`
- Create: `tools/release_hygiene_test.py` or equivalent focused test surface

**Interfaces:**
- Consumes: root Go build, desktop module, SPK builder/verifier, package-source generator.
- Produces: deterministic CI artifacts plus a bounded release-hygiene report that fails only on high-confidence secret/key material and reports machine/private-network references for human review.

- [ ] **Step 1: Add the focused failing tests**

Add release-hygiene fixtures proving:
- private-key headers, GitHub/OpenAI-style tokens, and obvious non-placeholder secret assignments fail;
- documented placeholders/examples do not fail;
- Windows user paths, `D:\VERA`, and RFC1918 addresses are reported as review findings rather than automatically classified as secrets.

Update CI assertions so the Windows job requires both root agent and desktop module tests/builds.

- [ ] **Step 2: Verify the relevant failure**

Run:
`python tools/release_hygiene.py --check .`

Expected before implementation: command/file absent.

Re-run the existing package-source provenance step against the exact branch head.

Expected at the current PR #2 head: failure because the checked-in package-source artifact/manifest does not match the CI-built SPK.

- [ ] **Step 3: Implement the minimum behavior**

Pin the root CI Go toolchain to one exact version. Prefer the already locally qualified Go `1.27.0` subject unless exact-head reproduction disproves it.

Regenerate the ARMv7 binary/SPK and package-source from that same pinned toolchain. Do not hand-edit checksums.

Add desktop CI:
- install Wails CLI exactly `v2.14.0`;
- run desktop tests;
- build the companion with `-webview2 browser`;
- upload the companion as a separate artifact.

Add the source-visible proprietary license/contribution/security files using the owner's established repository terms, with Tattler-specific NOTICE text.

Implement the hygiene scanner with explicit high-confidence blockers and review-only path/network findings.

- [ ] **Step 4: Verify the focused pass**

Run:
- `python tools/release_hygiene.py --check .`
- `node package-source/test.js`
- `python tools/verify_spk.py dist/Tattler.spk`

Expected: no high-confidence secret blocker; package-source tests and SPK verifier pass. Review-only path/network findings are enumerated and dispositioned.

- [ ] **Step 5: Run the affected integration check**

Run the exact GitHub Actions-equivalent build locally where supported, then push the exact head.

Expected CI:
- Ubuntu `test-and-package`: SUCCESS including byte-for-byte package-source binding;
- Windows `windows-agent` or renamed Windows release job: SUCCESS including native agent runtime smoke and companion build;
- artifacts for DSM SPK, Windows agent, and Windows companion are present.

- [ ] **Step 6: Commit the passing deliverable**

Commit message:
`build: harden public release qualification`

---

### Task 4: Align README/docs and qualify the exact release candidate

**Files:**
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/ROADMAP.md`
- Update/Create: exact-head qualification note under `docs/` only after tests/CI actually support the claims
- Modify: PR #2 title/body as needed to describe the final exact head and evidence ceiling

**Interfaces:**
- Consumes: implemented observation kinds, desktop module commands/artifacts, CI results, release-hygiene report.
- Produces: public documentation that a new user can build/run without reading historical state files and that does not promote source/build evidence into runtime/release claims.

- [ ] **Step 1: Add documentation consistency checks**

Add focused script/test assertions for:
- README no longer calls the complete mixed snapshot merely “connections”;
- README lists TCP sessions, TCP listeners, UDP endpoints, and their evidence limits;
- Windows companion build/run instructions point to `desktop/`;
- README's candidate version and package-source manifest version agree;
- status labels do not claim CI success until the exact head has it.

- [ ] **Step 2: Verify the relevant failure**

Run the documentation/release check against the current PR #2 head.

Expected: failure because Windows UDP and desktop companion are documented as absent and CI status text is stale.

- [ ] **Step 3: Implement the minimum documentation update**

Rewrite the README opening/quick start so a public user sees:
- what Tattler does;
- supported Linux/DSM and Windows surfaces;
- exactly what network observations mean;
- how to build/run the Windows agent and desktop companion;
- DSM package path;
- privacy/safety boundary;
- known sampling/attribution limitations;
- licensing and contribution pointers.

Update architecture diagrams/text for the two platform collectors and separate desktop consumer.

Update roadmap so implemented release work moves out of future items; keep ETW/high-fidelity capture as a later optional collector.

Create an exact-head qualification note only with evidence actually observed from the final head.

- [ ] **Step 4: Verify the focused pass**

Run:
`go test ./... && go vet ./...`
`cd desktop && go test ./...`
and the release/documentation checks.

Expected: PASS.

- [ ] **Step 5: Run exact-head published verification**

After pushing the candidate, read back the branch head, PR head, and workflow runs. Require the tested workflow commit SHA to equal the candidate head.

Expected: all required jobs SUCCESS for that exact SHA.

Then run one bounded Cricket hostile review over the exact candidate's public claims, API semantics, release evidence, and authority boundaries. Cricket critique is review evidence, not executable verification.

- [ ] **Step 6: Integrate only after a separately routed merge gate**

If exact-head acceptance is green, route the merge/main-update step through P.O.R.T.A.L. using the live user's already expressed intent that `main` be brought up to date. Re-read `main` after merge and verify it equals or contains the qualified candidate.

Do not create a GitHub Release or install/upgrade the live DS216 unless separately requested.

Commit message for documentation changes:
`docs: prepare Tattler public release`

## Unresolved externally observable decisions

None that block this plan. The WebView2 missing-runtime strategy is a reversible engineering recommendation (`browser`) rather than a product contract; changing it later does not alter the agent/desktop architecture.
