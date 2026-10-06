# Tattler Public Release Network + Desktop Design

Status: APPROVED DESIGN — implementation subject is `thebrazenbeard/tattler@build/windows-native-v1`.

## Goal

Prepare Tattler for a credible public release by broadening network visibility beyond the current Windows TCP-only collector, exposing the observations with honest typed semantics, adding a native Windows desktop companion over the existing loopback API, and closing release/documentation/provenance gaps without weakening the small DSM/Linux agent.

## Evidence boundary

Tattler remains a host-observation tool, not a packet sniffer or kernel tracing engine.

The public product term is **network activity / endpoint telemetry**. The model distinguishes:

- `tcp_session`: a non-listening TCP socket row representing a sampled TCP session state;
- `tcp_listener`: a sampled local TCP listening endpoint;
- `udp_endpoint`: a sampled local UDP endpoint. A remote peer is not claimed unless the platform evidence actually exposes one.

Sampling can miss short-lived activity. UDP endpoint presence does not prove datagrams were transmitted or received. Windows IP Helper endpoint tables do not provide packet payloads or complete peer history. ETW is explicitly outside this release.

## Observation model

Extend the existing `model.Connection` JSON shape rather than replacing the API:

- add `kind` with the values above;
- retain `protocol`, `local`, `remote`, `state`, ownership, process, and direction fields;
- keep `remote` as the platform table's zero/unspecified endpoint when no peer is represented;
- use `direction` only where supported: `inbound`/`outbound` for TCP sessions and `listen` for TCP listeners; omit direction for UDP endpoints;
- make the tracker key include observation kind and the best available ownership identity so two Windows UDP endpoints sharing a local tuple do not collapse solely because inode is unavailable.

The existing event envelope remains compatible. `open` and `close` mean an observation appeared in or disappeared from the sampled endpoint set; documentation must not redefine those as packet-level connection establishment/termination.

The status API adds `current_observations` while retaining `current_connections` as a compatibility alias during this release.

## Linux collector

Keep the existing `/proc/net/{tcp,tcp6,udp,udp6}` source and privilege model.

- TCP LISTEN rows become visible `tcp_listener` observations instead of being used only internally for direction classification.
- Non-listening TCP rows become `tcp_session`.
- UDP rows are retained even when the remote tuple is unspecified and become `udp_endpoint`.
- Existing inode/UID/account/process decoration applies to all observation kinds where the evidence exists.
- TCP direction continues to be derived from local-address membership and contemporaneous listener evidence.
- UDP direction is left unset rather than inferred.

## Windows collector

Extend the existing IP Helper backend.

TCP continues to use `GetExtendedTcpTable`, but listener rows become first-class `tcp_listener` observations rather than being discarded after classification.

Add IPv4 and IPv6 UDP enumeration using `GetExtendedUdpTable` with `UDP_TABLE_OWNER_PID`. Each row yields:

- local address/port;
- exact owning PID when Windows supplies it;
- best-effort process name/executable through the existing process-enrichment path;
- `kind=udp_endpoint`;
- no invented remote peer or direction.

A failure in one address-family/protocol table is joined with other collection errors while successfully collected tables remain visible, matching the existing partial-collection posture.

## Desktop companion

Create a separate Windows-only Wails v2 module under `desktop/`, pinned to the latest confirmed stable v2 release used by the implementation plan. The agent root module remains dependency-free.

The companion:

- uses a static HTML/CSS/JS frontend with no Node package dependency;
- binds a small Go application service to the Wails frontend;
- reads status/current/findings/events through `http://127.0.0.1:9147` from Go, avoiding broad CORS changes to the agent;
- on startup, attaches to a healthy existing Tattler agent if present;
- if the agent is absent, may start only a sibling Tattler Windows agent executable using a state directory under the current user's local application-data/cache location;
- records whether it owns the child process;
- on exit, terminates only a child process it launched; it never stops an independently running Tattler instance;
- displays explicit unavailable/offline state instead of manufacturing telemetry.

The first companion release is Windows-only. Cross-platform desktop packaging is not implied.

## Release packaging

GitHub Actions must continue to prove the Linux/DSM agent separately from the Windows agent and desktop companion.

The Windows job will:

- run root-module tests/vet;
- exercise native TCP and UDP collection tests;
- build the agent reproducibly;
- build the Wails companion with pinned Wails v2 tooling;
- start the built agent and validate the loopback API;
- exercise the companion backend against the live loopback API;
- publish agent and companion build artifacts.

The DSM job must regenerate package-source metadata/artifacts from the exact CI-built SPK and compare them byte-for-byte. The current PR #2 failure is a real provenance failure and must be fixed by source/toolchain/artifact alignment, not by weakening the check.

## Public-release hygiene

Add the repository's public licensing/contribution/security surface consistent with the owner's other source-visible repositories:

- `LICENSE`
- `COMMERCIAL_LICENSE.md`
- `NOTICE`
- `CONTRIBUTING.md`
- `CLA.md`
- `SECURITY.md`

Update `README.md`, `docs/ARCHITECTURE.md`, and `docs/ROADMAP.md` so current behavior, evidence ceilings, build instructions, desktop packaging, and qualification state agree.

Run a branch-wide text audit for credential-like strings, private keys, machine-specific private paths, and accidental private network/host details before merge. Findings are reviewed, not blindly deleted: historical qualification material may legitimately contain bounded environment facts but secrets must not remain.

## Release gate

A release candidate is source-ready only when the exact candidate head has:

- root Go tests and vet passing on Linux and Windows;
- focused Linux and Windows TCP-listener/UDP endpoint tests passing;
- deterministic agent builds passing where currently required;
- deterministic DSM package build and strict verifier passing;
- package-source provenance comparison passing;
- desktop companion build passing on Windows;
- native Windows loopback runtime smoke passing;
- public-release hygiene scan reviewed;
- README/architecture/roadmap consistent with the exact head.

Source readiness does not establish DSM installation/runtime qualification, ETW coverage, packet capture, release publication, or any other untested effect.
