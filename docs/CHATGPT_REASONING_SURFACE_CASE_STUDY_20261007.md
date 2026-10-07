# ChatGPT Reasoning-Surface Case Study — 2026-10-07

Status: OBSERVATION CASE STUDY / NOT A PROVIDER-INTERNAL CLAIM

## Purpose

This case study records a concrete Windows use of Tattler's protocol-evidence branch during a same-task comparison of ChatGPT reasoning surfaces.

It exists to sharpen Tattler's evidence model, not to turn Tattler into a model detector.

## Tattler subject

The experiment used the exact Windows CI artifact associated with:

- repository: `thebrazenbeard/tattler`
- feature branch: `feat/protocol-evidence-v1-20261006`
- qualified source commit: `2999adf5fac0e00ab4e82ce06c7de6ae7273fd3b`
- qualification workflow run: `37552266918`
- artifact id: `11453415564`
- artifact name: `tattler-windows-amd64`
- live poll interval: `250ms`
- live metrics interval: `1s`

The executable was run on WorkLaptop with a fresh state directory for each controlled phase.

A separate companion tracer observed process births/command lines and connection transitions so that MXC launches could be counted independently of Tattler's own journal semantics.

## Task

All three surfaces received the same user instruction:

```text
Stress test this repo: https://github.com/thebrazenbeard/portal
```

Surfaces:

1. Desktop Chat — GPT-5.6 Sol High reasoning
2. ChatGPT Desktop Work — Ultra reasoning
3. Firefox cloud Work — Max reasoning

## High window

Markers:

- start: `2026-10-07T12:38:06.1331014Z`
- end: `2026-10-07T13:00:45.3042022Z`

Companion-tracer observations:

- 0 MXC launches;
- 0 fs-helper launches;
- 2 new established Codex TLS connections;
- both remote endpoints: `104.18.32.47:443`.

Tattler recorded transport activity for ChatGPT/Codex but received no application-reported semantic events.

## Desktop Work Ultra window

Markers:

- start: `2026-10-07T13:06:29.4952049Z`
- end: `2026-10-07T13:27:09.9895098Z`

Companion-tracer observations:

- 59 MXC launches;
- 0 fs-helper launches;
- 73 new established Codex TLS connections;
- 45 to `104.18.32.47:443`;
- 28 to `172.64.155.209:443`.

Tattler itself recorded a larger number of Codex outbound established-open events because its polling/journal semantics differ from the companion tracer. The two counts are not interchangeable.

The controlled High-vs-Ultra comparison therefore uses the companion tracer's like-for-like event definitions for the process/socket headline.

## Firefox Work Max window

The user started Firefox Work Max slightly before the marker:

`2026-10-07T13:15:28.7909687Z`

Max completed at:

`2026-10-07T13:37:17.6413079Z`

This overlapped Desktop Ultra until Ultra ended at `13:27:09.9895098Z`.

Tattler could observe Firefox-side network activity, but those local TLS sessions do not expose the provider's cloud worker count, agent topology, reasoning trace, or internal fanout.

The late start marker also means the exact first seconds of Max activity are intentionally ambiguous.

## What Tattler legitimately helped establish

Tattler plus the companion process tracer supports this bounded statement:

> On this Windows runtime and task, Desktop Work Ultra exhibited substantial local Codex/MXC and transport fanout that was absent from the controlled High Chat window.

It does not support:

- one socket = one agent;
- one MXC process = one reasoning worker;
- socket count = reasoning depth;
- a network signature that grants or proves a model entitlement;
- recovery of hidden chain-of-thought;
- inference of Max cloud worker count from Firefox traffic.

## Protocol-evidence lessons

### Port-derived evidence remains heuristic

Remote port 443 supports labels such as `tls?` and `https?` only at the documented heuristic confidence. No TLS decryption or ALPN proof occurred.

### Application semantics require reporters

The `/api/v1/semantic-events` surface remained empty for ChatGPT/Codex because no local adapter posted application-level events.

This is useful evidence: Tattler should not silently convert encrypted transport observations into semantic claims.

### Process ownership and provider semantics are separate

Knowing that `codex.exe` owns a connection does not establish:

- which model produced a response;
- which reasoning tier was selected;
- whether a child process corresponds to an agent;
- what task the connection carried.

### Cloud topology has a hard local ceiling

For browser-based Work Max, Tattler can describe the browser's local endpoint activity but not the remote provider's internal orchestration.

## Product implication

Tattler is valuable for **reasoning-surface observability** when claims are kept at the transport/process layer.

A future optional adapter could improve semantic correlation by explicitly reporting bounded local events such as:

- user-declared phase start/end;
- selected product surface as reported by the local UI/adapter;
- task/receipt identifiers;
- locally observed tool invocation boundaries.

Such a reporter must remain an application assertion with provenance. It must not be presented as something Tattler reconstructed from encrypted traffic.

## Claim ceiling

This case study is a single-machine black-box observation. It does not establish stable provider signatures across versions, universal ChatGPT behavior, hidden model topology, or a method for increasing reasoning capability.
