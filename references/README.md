# Ingested Reference Layer

This directory makes reference provenance explicit without copying third-party code into this repository.

## Policy

- `SOURCE.md` records origin URL, intended use, licensing follow-up, and ingestion status.
- `behavior.md` records the behavior extracted into this project’s independent specification.
- `raw/` is reserved for immutable source snapshots, release tarballs, or excerpts only after the source version, checksum, and license/reuse review have been completed.
- `fixtures/` holds generated oracle outputs, their generator, tool version, and tolerances. No network call is permitted in normal CI.

The current implementation uses the Welford and Chan-style mathematics from the project wiki. It has **no copied third-party implementation code** and intentionally has no p-value calculation until the Gonum distribution adapter is independently validated.

See `wiki/06-reference-implementation-register.md` for the full registry.
