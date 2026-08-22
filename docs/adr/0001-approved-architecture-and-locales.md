# ADR 0001: Approved architecture and bilingual locales

- Status: Accepted
- Date: 2026-08-20
- Decision owner: Project owner

## Context

The project owner approved `ARCHITECTURE.md` and authorized implementation. The owner additionally requires the system to operate in Thai and English.

## Decision

1. Architecture decisions 1–10 in `ARCHITECTURE.md` are accepted.
2. The product supports locale codes `th` and `en`; deployment default is `th`.
3. Business state, error, audit and recommendation reason codes remain stable English identifiers.
4. Human-readable system messages come from a versioned translation catalog rather than duplicated localized business rows.
5. API locale precedence is explicit request/header, authenticated user preference, deployment default.
6. User-entered data is preserved verbatim and is not automatically translated.

## Consequences

- Phase 1 includes locale negotiation, bilingual error messages and locale metadata.
- Dashboard language selection is implemented in Phase 7 using the same codes/catalog contract.
- New user-facing codes require both Thai and English translations in tests.
