# Tattler architecture V0.1

## Product question

Tattler is an observation-only diagnostic agent. Its primary question is not merely "what is connected?" but "why is this Linux/NAS host slow right now, and what host evidence supports that answer?"

## Evidence pipelines

```text
/proc/loadavg /proc/stat /proc/meminfo /proc/vmstat /proc/diskstats
                         |
                         v
                 system sampler
                         |
             +-----------+-----------+
             v                       v
      bounded history           diagnosis rules
                                    |
                                    v
                         evidence-bearing findings

/proc/net/{tcp,tcp6,udp,udp6}
          |
          v
  socket-table parser ------ /proc/<pid>/fd (best effort)
          |                           |
          +------------+--------------+
                       v
              attributed snapshot
                       |
             local/listener classifier
                       |
                snapshot differ
                       |
             open / close events
                 /           \
                v             v
        rotated JSONL     memory window

                         |
                         v
                loopback HTTP/API/UI
```

Connection sampling defaults to one second. The more expensive whole-host/process sampler defaults to five seconds so the diagnostic agent stays small on a 512 MB DS216.

## System telemetry

The system sampler reads Linux kernel text interfaces rather than depending on a resident metrics stack. V0.1 measures load averages, aggregate CPU utilization, I/O-wait share, memory availability, swap use and churn, major faults, aggregate physical-disk read/write rates, runnable-process count, and a bounded list of resource-heavy processes.

Process CPU is derived from per-process tick deltas against aggregate CPU tick deltas. Process I/O uses `/proc/<pid>/io` where readable. Missing process data is treated as missing evidence rather than as zero-cost proof.

The diagnosis layer currently recognizes pressure patterns, not root causes: `memory-pressure`, `memory-critical`, `swap-churn`, `storage-wait`, `cpu-saturation`, `blocked-load`, and `major-faults`. Each finding carries the measurement that triggered it.

## Connection event contract

Each connection event has `schema_version`, `event_id`, `observed_at`, `host`, `kind`, `connection`, and `source`. `event_id` is SHA-256 over the exact V1 occurrence fields. `source=proc-sampler` describes how the observation was made; it does not claim kernel packet telemetry.

Connection records include protocol, local/remote endpoints, state when available, direction, socket inode, UID, and best-effort process attribution. TCP direction is derived from local-address membership plus contemporaneous listening sockets. UDP evidence is materially weaker and must not be interpreted as complete datagram provenance.

## Safety boundary

V0.1 is read-only. It does not modify firewall/routing state, open raw packet sockets, capture payloads, alter DNS, create public listeners, or transmit telemetry off-host. The DSM package runs as the package account rather than root.

The UI/API is hard-bound to loopback. Persistent connection-event state is created with restrictive permissions inside package state. System-sample history is currently memory-bounded and intentionally not written every five seconds to avoid turning Tattler into a source of disk pressure on the DS216.

## Known limitations

- Sampling can miss short connections and short resource spikes between samples.
- PID/process attribution can fail because a process exited or DSM denies `/proc/<pid>/fd`.
- Linux network namespaces can hide sockets/processes from the host namespace view.
- NAT can make observed endpoints differ from external/application endpoints.
- Aggregate disk throughput does not by itself reveal latency, queue depth, SMART health, or which block layer caused a stall.
- I/O-wait is evidence of tasks waiting on I/O, not proof of a failing disk.
- Reverse DNS and SMART polling are intentionally absent from the hot path.

## Extension seams

The connection journal/API consumes versioned events rather than raw `/proc` rows, so a later collector can add Netfilter conntrack, pcap, or eBPF without replacing the data contract.

The system sampler is similarly independent of diagnosis rules. Future SMART/RAID/DSM-specific collectors can enrich a sample or produce separate evidence without allowing one source to silently stand in for another.
