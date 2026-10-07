# Tattler architecture

## Product question

Tattler is an observation-only diagnostic agent. Its primary question is: “why is this host slow right now, and what sampled host evidence supports that answer?”

Linux/DSM and Windows use platform-specific collectors but publish into one evidence model. The Windows desktop companion is a consumer of that local evidence; it is not a second collector.

## Evidence pipelines

```text
Linux /proc system/process sources             Windows system APIs
              |                                      |
              v                                      v
        system sampler                         system sampler
              |                                      |
              +------------------+-------------------+
                                 |
                     bounded system history
                                 |
                         diagnosis rules
                                 |
                    evidence-bearing findings

Linux /proc/net                         Windows IP Helper
tcp,tcp6,udp,udp6              TCP/UDP OWNER_PID tables
        |                                |
        +---------------+----------------+
                        v
              typed transport snapshot
        tcp_session / tcp_listener /
          udp_endpoint / udp_flow
                        |
       ownership + protocol evidence
                        |
                snapshot differ
                        |
       sampled open / close observation events
                 /                 \
                v                   v
         rotated JSONL        memory window
                 \                 /
                  +-------+---------+
                          v
                  loopback HTTP/API/UI
                          |
             +------------+------------+
             |                         |
       browser dashboard        Windows desktop
                               Wails companion

Local apps / proxies / webhook workers
                 |
      loopback JSON semantic reports
                 |
        bounded semantic memory
                 |
                 +-----> loopback HTTP/API/UI
```

Network sampling defaults to one second. The more expensive Linux whole-host/process sampler defaults to five seconds so the agent remains small on DS216-class hardware.

## Typed network observations

The public transport model uses four kinds:

- `tcp_session`: a sampled non-listening TCP endpoint pair and state;
- `tcp_listener`: a sampled local TCP listener;
- `udp_endpoint`: a sampled bound UDP endpoint without a proven remote peer;
- `udp_flow`: a Linux connected-UDP row whose sampled kernel table exposes a nonzero remote endpoint.

Linux reads `/proc/net/{tcp,tcp6,udp,udp6}`. Windows uses `GetExtendedTcpTable` and `GetExtendedUdpTable` OWNER_PID tables. Windows IP Helper UDP tables do not expose remote peers, so Windows UDP remains `udp_endpoint` evidence in V1.

An `udp_endpoint` does not prove a datagram was sent or received. A Linux `udp_flow` proves only that the kernel table exposed the tuple at sample time, not that Tattler observed datagram traffic or a handshake. Likewise, a `tcp_session` is a sampled table row, not proof that Tattler observed the handshake.

TCP direction is derived from local-address membership plus contemporaneous listener evidence where the platform supplies enough information. Listener direction is `listen`. UDP direction remains unset unless a future stronger collector proves it.

Each connection can also carry `protocol_evidence`. V1 port-derived entries use `confidence=heuristic` and `source=well_known_port`. Transport remains unchanged: TCP/443 is still transport `tcp` even when Tattler adds `tls ?` and `https ?`; UDP/443 is still `udp`/`udp_flow` even when Tattler adds `quic ?`. V1 never promotes UDP/443 into an HTTP/3 claim because it does not parse QUIC ALPN.

## Observation event contract

Each event has `schema_version`, `event_id`, `observed_at`, `host`, `kind`, `connection`, and `source`. The connection record also carries its typed observation `kind`.

Journal event names `open` and `close` mean appearance and disappearance between samples. They do not claim kernel socket lifecycle, TCP SYN/FIN, or UDP datagram events.

`event_id` is deterministic over the V1 occurrence fields. Collector `source` identifies how the evidence was sampled; it is provenance, not a fidelity upgrade.

## Reported semantic evidence

The loopback API exposes `/api/v1/semantic-events` as a separate application-semantic evidence path.

Local applications, reverse proxies, webhook handlers, or adapters can report:

- `http_transaction`;
- `webhook_delivery`;
- `websocket_session`;
- `grpc_rpc`.

These reports use `confidence=reported` and `source=reported`. That is provenance: Tattler records what the local reporter asserted but does not independently decrypt or reconstruct the transaction.

The POST schema is deliberately metadata-only. It does not accept raw request/response bodies, arbitrary headers, cookies, authorization values, raw URLs, query strings, fragments, or arbitrary error text. `route` is constrained to a low-cardinality path/template and `error_type` to a bounded token.

Semantic reports are memory-bounded and session-scoped in V1. They are not written into the durable connection-event JSONL journal because that journal has a different schema and recovery contract.

## System telemetry

The Linux sampler reads kernel text interfaces rather than requiring a resident metrics stack. It measures load averages, aggregate CPU utilization, I/O-wait share, memory availability, swap use/churn, major faults, physical-disk rates/latency/utilization/queue evidence, Linux MD state, runnable-process count, and a bounded list of resource-heavy processes.

The Windows sampler currently provides host CPU and physical-memory evidence. Linux-only fields remain explicitly unavailable on Windows rather than being inferred.

Process CPU on Linux is derived from per-process tick deltas against aggregate CPU tick deltas. Process I/O uses `/proc/<pid>/io` where readable. Missing process data is missing evidence, not zero.

The diagnosis layer recognizes bounded pressure patterns such as `memory-pressure`, `memory-critical`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, `major-faults`, and proven MD degradation. Each finding carries the measurements that triggered it.

## Windows desktop companion

The optional `desktop/` module uses Wails v2.14.0 and static frontend assets. Its Go backend reads only the agent’s loopback API at `127.0.0.1:9147`.

On startup it attaches to an existing healthy agent. If no agent is available, it may launch a sibling Windows Tattler executable and records ownership of only that child. Shutdown may stop only a child the companion launched itself.

The companion does not open a public listener, collect network telemetry independently, or broaden the agent’s authority.

## Safety boundary

Tattler does not modify firewall/routing state, open raw packet sockets, capture payloads, decrypt TLS/QUIC, alter DNS, create public listeners, or transmit telemetry off-host. The DSM package runs as the package account rather than root.

The HTTP UI/API is hard-bound to loopback. Semantic POSTs require `application/json`, and the schema excludes payload/credential-bearing fields. Persistent connection-event state is created inside the configured state directory. System-sample and semantic-event histories are memory-bounded to avoid making Tattler itself a source of disk pressure.

## Known limitations

- Sampling can miss short TCP sessions, endpoint churn, and short resource spikes between samples.
- UDP endpoint presence is not datagram or remote-peer evidence.
- Linux `udp_flow` presence is sampled socket-tuple evidence, not datagram/handshake evidence.
- Port-derived protocol names are heuristics and can be wrong when applications use nonstandard or repurposed ports.
- Windows IP Helper does not expose remote UDP peers, so outbound Windows QUIC cannot be honestly reconstructed from this collector alone.
- Reported semantic events are assertions from local reporters and are not independently verified by Tattler.
- PID/process attribution can fail because a process exited or OS permissions deny enrichment.
- Linux network namespaces can hide sockets/processes from the host namespace view.
- NAT can make observed endpoints differ from application/external endpoints.
- I/O-wait and storage latency are evidence of waiting, not proof of a failing disk.
- ETW, pcap, eBPF, reverse DNS, and hot-path SMART polling are not part of this release.

## Extension seams

The journal/API consumes versioned typed observations rather than raw platform rows, so later ETW, Netfilter conntrack, pcap, or eBPF collectors can be added without pretending their evidence is equivalent to sampled tables.

The system sampler is independent of diagnosis rules. Future DSM/device identity, low-frequency SMART, and richer Windows telemetry can enrich the model without allowing one source to silently stand in for another.
