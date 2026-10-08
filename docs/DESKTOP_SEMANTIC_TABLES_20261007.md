# Desktop semantic tables — approved source-only implementation

Base lineage: unmerged Tattler PR #6 `2640d4af1e69f3eb48b002b8270e2b1298169937` → Stage 1 tracking/freshness branch → Stage 2 desktop branch. No merge, install, release, live runtime behavior, or qualification is asserted.

- Four independent accessible native `details/summary` tables: Findings, Network activity, Semantic activity, Recent observation events. Open states persist locally; network expands by default, events default collapsed.
- Network semantic views are evidence-bound: TCP states, listening endpoints, UDP endpoints/flows, known non-loopback remote peers, loopback-bound, and unknown owner. A non-loopback peer is not claimed to be the public Internet; wildcard listeners are not declared publicly exposed.
- Process dropdown, search and sort operate on cached sampled observations without further agent queries; filter chips remove active view/process/search constraints.
- Grouped view aggregates similar process/kind/direction/state/remote endpoint observations, but preserves raw evidence in expandable rows. Group totals count **raw sampled rows** and **distinct tracking keys** when the agent provides tracking-key metadata; these are never asserted to count distinct kernel sockets.
- Raw row details contain source fields, identity basis, full local/remote endpoint and process/owner information. No extra collector privileges are required.
- The separate semantic table displays **reported** application events; port heuristics are marked `?` and never silently upgraded into traffic proof.
- Fast status/network reads target 2 seconds; findings 5 seconds; events/semantic reports 10 seconds. No overlapping calls per section; hidden-window reads are reduced; cached section bodies are not repainted when collapsed or unchanged. Elapsed ages tick client-side.
- If a collector fails, the desktop retains cached evidence with explicit incomplete/last-complete labels rather than presenting old rows as current. Optional semantic API remains compatible with older agents.
- Presentation/source checks cannot establish live desktop user-flow correctness, responsiveness on slow hardware, or actual reduced CPU/memory/API load. These need qualified runtime observation.
