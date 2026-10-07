# Tattler protocol and semantic evidence 0.2.0-0005 qualification — 2026-10-06

## Exact subject

Source/package subject:

`thebrazenbeard/tattler@2999adf5fac0e00ab4e82ce06c7de6ae7273fd3b`

Pull request:

`#6 — feat/protocol-evidence-v1-20261006`

GitHub Actions qualification:

- PR run `37552266918`: SUCCESS
- push run `37552263755`: SUCCESS

Both runs executed against the exact subject above.

## Package family

Package version: `0.2.0-0005`

DSM minimum build: `72806`

Canonical Package Source artifacts:

- `Tattler-x86_64-0.2.0-0005.spk` — ELF e_machine `62` — 3,194,880 bytes — SHA-256 `3d4c95796e3ff955641db0e7c7db529af14a101876ea66084372cf30dc738273`
- `Tattler-armv7-0.2.0-0005.spk` — ELF e_machine `40` — 3,102,720 bytes — SHA-256 `545d6f5107a542642c04a123fc5a11ca1d1f2f670c2506aadafee3b9638dc8ef`
- `Tattler-armv8-0.2.0-0005.spk` — ELF e_machine `183` — 2,887,680 bytes — SHA-256 `651b9650d80590570f2b54a72a6a9c6e7e3a52ab9a11d57a23f6836ec56f43df`

The checked-in Package Source copies are byte-for-byte identical to the CI-built SPKs for this exact subject.

## Qualified transport and protocol evidence

The public transport model includes `tcp_session`, `tcp_listener`, `udp_endpoint`, and `udp_flow`.

Linux `udp_flow` is emitted only when sampled `/proc/net/udp*` exposes a nonzero remote endpoint. It is sampled socket-tuple evidence, not proof that Tattler observed a datagram or handshake.

Windows IP Helper UDP rows remain `udp_endpoint` because the OWNER_PID table does not expose remote UDP peers.

Connections can carry separate `protocol_evidence`. V1 well-known-port entries use `confidence=heuristic` and `source=well_known_port`.

Qualified heuristic names include HTTP/HTTPS/TLS, QUIC, DNS/DoT/DoQ, mDNS, NTP, SSDP, SNMP/SNMP trap, DHCP, SSH, SMB, and NFS.

Claim ceilings:

- UDP/443 may yield `quic ?`; it does not establish HTTP/3.
- TCP/443 may yield `tls ?` and `https ?`; it does not establish that an HTTP request occurred.
- Port heuristics can be wrong when software uses nonstandard or repurposed ports.

## Reported semantic evidence

The loopback API includes `GET/POST /api/v1/semantic-events`.

Supported reported kinds are `http_transaction`, `webhook_delivery`, `websocket_session`, and `grpc_rpc`.

Reported events use `confidence=reported` and `source=reported`. A local app/proxy/adapter asserted the metadata; Tattler did not decrypt or independently reconstruct the transaction.

POST requires `Content-Type: application/json`, rejects unknown fields, is body-size bounded, and intentionally has no schema fields for raw bodies, arbitrary headers, cookies, authorization values, raw URLs, query strings, fragments, or arbitrary error messages.

Semantic events are memory-bounded and session-scoped in V1 and are not mixed into the durable connection-event JSONL journal.

## UI and compatibility

Browser and Windows desktop surfaces distinguish Transport, Protocol evidence, and Semantic activity. Heuristic evidence renders with a `?` marker.

CI syntax-checks both JavaScript surfaces. The Windows desktop treats a missing semantic endpoint on an older agent as an empty optional collection instead of failing the entire snapshot.

## Verification

The exact-head workflows passed:

- all Go tests and Go vet;
- protocol-classifier and Linux connected-UDP regressions;
- semantic API validation/privacy/bounding tests;
- release-document tests;
- UI JavaScript syntax tests;
- desktop packaging tests;
- release hygiene with zero blockers;
- deterministic x86_64, ARMv7, and ARMv8 binary double builds;
- deterministic three-architecture SPK double builds;
- in-repo and independent DSM 7.2.2 SPK verification for all three;
- Package Source routing and exact byte binding;
- native Windows runtime smoke;
- Windows desktop tests/build/bundle.

Supplemental live Windows development readback from the same agent source showed a healthy loopback API; a sample of 317 current rows with 121 carrying protocol evidence; HTTPS/TLS, SMB, DNS, NTP, mDNS, DHCP, and SSDP heuristic evidence; successful reported webhook ingestion; and a fresh semantic session returning JSON `[]`.

## Evidence ceiling

This qualification does not establish packet capture, payload inspection, TLS/QUIC decryption, QUIC ALPN parsing, confirmed HTTP/3 from UDP/443, remote-peer visibility for outbound Windows UDP/QUIC through the current IP Helper collector, ETW/WFP/eBPF/conntrack collection, installation of `0.2.0-0005` on the live DS216, or live DSM runtime behavior for `0.2.0-0005`.

Full Windows UDP/QUIC flow visibility requires a separately implemented and qualified stronger Windows telemetry source. DSM installation/runtime remains a separate protected subject.
