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
              typed endpoint snapshot
        tcp_session / tcp_listener /
                   udp_endpoint
                        |
             ownership enrichment
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
```

Network sampling defaults to one second. The more expensive Linux whole-host/process sampler defaults to five seconds so the agent remains small on DS216-class hardware.

## Typed network observations

The public model uses three kinds:

- `tcp_session`: a sampled non-listening TCP endpoint pair and state;
- `tcp_listener`: a sampled local TCP listener;
- `udp_endpoint`: a sampled bound UDP endpoint.

Linux reads `/proc/net/{tcp,tcp6,udp,udp6}`. Windows uses `GetExtendedTcpTable` and `GetExtendedUdpTable` OWNER_PID tables.

A UDP endpoint has no inferred remote peer or direction. Its presence does not prove a datagram was sent or received. Likewise, a `tcp_session` is a sampled table row, not proof that Tattler observed the handshake.

TCP direction is derived from local-address membership plus contemporaneous listener evidence where the platform supplies enough information. Listener direction is `listen`. UDP direction remains unset.

## Observation event contract

Each event has `schema_version`, `event_id`, `observed_at`, `host`, `kind`, `connection`, and `source`. The connection record also carries its typed observation `kind`.

Journal event names `open` and `close` mean appearance and disappearance between samples. They do not claim kernel socket lifecycle, TCP SYN/FIN, or UDP datagram events.

`event_id` is deterministic over the V1 occurrence fields. Collector `source` identifies how the evidence was sampled; it is provenance, not a fidelity upgrade.

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

Tattler does not modify firewall/routing state, open raw packet sockets, capture payloads, alter DNS, create public listeners, or transmit telemetry off-host. The DSM package runs as the package account rather than root.

The HTTP UI/API is hard-bound to loopback. Persistent event state is created inside the configured state directory. System-sample history is memory-bounded to avoid making Tattler itself a source of disk pressure.

## Known limitations

- Sampling can miss short TCP sessions, endpoint churn, and short resource spikes between samples.
- UDP endpoint presence is not datagram or remote-peer evidence.
- PID/process attribution can fail because a process exited or OS permissions deny enrichment.
- Linux network namespaces can hide sockets/processes from the host namespace view.
- NAT can make observed endpoints differ from application/external endpoints.
- I/O-wait and storage latency are evidence of waiting, not proof of a failing disk.
- ETW, pcap, eBPF, reverse DNS, and hot-path SMART polling are not part of this release.

## Extension seams

The journal/API consumes versioned typed observations rather than raw platform rows, so later ETW, Netfilter conntrack, pcap, or eBPF collectors can be added without pretending their evidence is equivalent to sampled tables.

The system sampler is independent of diagnosis rules. Future DSM/device identity, low-frequency SMART, and richer Windows telemetry can enrich the model without allowing one source to silently stand in for another.
