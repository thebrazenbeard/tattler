# Network identity, completeness, and freshness — source candidate

Source base: unmerged Tattler PR #6 at `2640d4af1e69f3eb48b002b8270e2b1298169937`. Not a public release or installed runtime.

- The Linux tracking key includes kind, protocol, both endpoints and inode when available. Windows falls back to PID, so distinct native rows may share a key. This is **not** a unique kernel socket ID.
- `/api/v1/current` deliberately retains collector raw-row multiplicity and exposes `tracking_key`, `identity_basis`, and `same_tracking_key_rows`; none proves distinct sockets. Row count and distinct keys are separate measures.
- The tracker emits at most one appearance event per new tracking key per accepted scan, avoiding identical event IDs produced by repeated raw rows.
- On collector error, an **incomplete** pass is rejected entirely. Last complete state remains cached and no appearance/disappearance diff is committed. This may delay legitimate changes from tables that did succeed: prefer false staleness over invented lifecycle events.
- `/api/v1/status.network_scan` reports last attempt, last complete pass, and incomplete scans skipped. The desktop must label stale data; never equate last known state with live state.
- A first complete pass establishes an event-free baseline, including after initially incomplete startup passes.
- Windows IPv6 table scope IDs remain part of displayed address identity, avoiding conflation of scoped endpoints.
- **Known limitation**: Linux `procnet.Read` historically treats missing optional `/proc/net/*` tables as absent capability, not necessarily an error. Thus `complete` means collector-reported complete, not proof that every theoretically supported network namespace/table is covered.
- Sampling misses short-lived endpoints; TCP open/close means sampled appearance/disappearance, not confirmed handshake/FIN; UDP endpoints do not establish peers or datagram transfer.
- This design does not deduplicate raw current rows, add kernel socket handles, introduce ETW/WFP/pcap, or claim live behavioral qualification.
