# Protocol and semantic evidence V1

## Purpose

Tattler separates transport evidence from application-protocol evidence so sampled sockets are never promoted into claims they cannot support.

A live row can now carry:

- transport identity: `tcp`, `tcp6`, `udp`, or `udp6`;
- observation kind: `tcp_session`, `tcp_listener`, `udp_endpoint`, or `udp_flow`;
- zero or more `protocol_evidence` entries with a name, layer, confidence, source, and reason.

The UI renders heuristic protocol evidence with a `?` suffix.

## Socket-derived protocol evidence

V1 uses well-known service ports only as **heuristics**. It does not inspect payloads.

Examples:

- TCP/80 or TCP/8080 → `http ?`
- TCP/443 or TCP/8443 → `tls ?`, `https ?`
- UDP/443 → `quic ?`
- TCP/53 or UDP/53 → `dns ?`
- TCP/853 → `tls ?`, `dot ?`
- UDP/853 → `quic ?`, `doq ?`
- UDP/5353 → `mdns ?`
- UDP/123 → `ntp ?`
- UDP/1900 → `ssdp ?`
- TCP/445 → `smb ?`
- TCP/2049 → `nfs ?`
- TCP/22 → `ssh ?`

Port evidence does **not** establish application payload content. In particular:

- UDP/443 does not prove HTTP/3. HTTP/3 is HTTP mapped over QUIC (RFC 9114), and the socket table does not expose the QUIC ALPN.
- TCP/443 does not prove HTTP semantics; it is reported only as a TLS/HTTPS heuristic.
- UDP/853 is a strong well-known-port hint for DNS over QUIC (RFC 9250), but V1 still labels it heuristic because no QUIC handshake is parsed.
- TCP/853 is a well-known-port hint for DNS over TLS (RFC 7858).
- UDP/5353 is the well-known mDNS port (RFC 6762).

### Linux UDP flow evidence

Linux `/proc/net/udp` and `/proc/net/udp6` can expose a nonzero remote endpoint for a connected UDP socket. Tattler represents those rows as `udp_flow`.

A `udp_flow` proves only that the kernel table exposed a local/remote UDP socket tuple at sample time. It does not prove that Tattler observed any datagram or protocol handshake.

### Windows UDP limitation

Windows IP Helper OWNER_PID UDP tables expose local endpoints and owner PID but not remote UDP peers. Those remain `udp_endpoint` observations.

Tattler does not invent outbound QUIC peers from Windows UDP endpoints. Full Windows QUIC-flow visibility requires a separate event/flow collector such as ETW or another qualified OS telemetry surface.

## Reported semantic evidence

Applications, reverse proxies, webhook handlers, or other local instrumentation can report transaction metadata to:

`POST http://127.0.0.1:9147/api/v1/semantic-events`

The endpoint accepts only `Content-Type: application/json`. The Tattler API remains loopback-only.

Supported semantic kinds:

- `http_transaction`
- `webhook_delivery`
- `websocket_session`
- `grpc_rpc`

Supported reported protocol names include `http`, `https`, `http1`, `http2`, `http3`, `websocket`, `websocket_tls`, `grpc`, and `grpcs`.

Reported semantic evidence is marked:

- `confidence: reported`
- `source: reported`

That means a local reporter asserted the metadata. Tattler did not independently decrypt or reconstruct the transaction.

### Webhook example

```json
{
  "kind": "webhook_delivery",
  "protocol": "https",
  "direction": "outbound",
  "reporter": "my-webhook-worker",
  "peer": "hooks.example.com:443",
  "method": "POST",
  "route": "/hooks/github",
  "status": 202,
  "duration_ms": 184,
  "bytes_out": 512,
  "provider": "github",
  "event_type": "push",
  "delivery_id": "delivery-123",
  "retry": 1,
  "signature_valid": true
}
```

### Privacy boundary

The semantic API deliberately has no fields for:

- request or response bodies;
- arbitrary headers;
- cookies;
- authorization values;
- raw URLs;
- query strings;
- fragments;
- arbitrary error messages.

`peer` must be a host or host:port, not a URL. `route` must be a low-cardinality route/template without query strings, fragments, or a URL scheme. `error_type` is a bounded token such as `timeout` or `connection_refused`.

This follows the same principle used by modern HTTP telemetry conventions: transaction metadata is useful, while URLs and query material can carry credentials or other sensitive data.

## Retention

Socket open/close observation events continue to use the durable rotated JSONL journal.

Semantic events are intentionally **session-scoped and memory-bounded** in V1. They are not silently mixed into the connection-event journal because that journal has a different schema and durability contract.

A later semantic journal can be added as a separate versioned persistence surface.

## Future confidence upgrades

The V1 model leaves room for future sources that can produce stronger evidence:

- Windows ETW;
- Windows Filtering Platform-derived qualified flow evidence;
- Linux conntrack;
- eBPF;
- explicit reverse-proxy integrations;
- OpenTelemetry adapters;
- protocol-aware collectors.

Those sources must state their own provenance and confidence. They must not silently upgrade a port heuristic into a confirmed protocol claim.
