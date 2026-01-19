# CODEX TASK (SINGLE TICKET IMPLEMENTATION)

## You MUST read these two documents first:
1) `design_doc.md` (system architecture, domain, ERD, flows)
2) `agents_codex.md` (coding rules, conventions, folder structure, testing, migrations)

## Objective
Implement the Jira ticket below EXACTLY, integrating into the existing codebase.

## Non-Negotiable Rules
- Follow `agents_codex.md` conventions and patterns.
- Do NOT introduce new frameworks or architectural styles.
- Do NOT duplicate existing helpers/types. Reuse what exists.
- Keep changes minimal and localized to what the ticket requires.
- Add tests and migrations when required by the ticket.
- If anything is missing, STOP and ask targeted questions (max 5).

## Reference Code Pointers (OPTIONAL, but use when provided)
Mimic these existing patterns and reuse these modules:
- Similar controller: <path>
- Similar service: <path>
- Similar repo: <path>
- Shared helpers to reuse: <path + name>
- Existing DTO/model shapes: <path>

If pointers are empty, you MUST locate similar code by searching the repo structure described in `agents_codex.md`.

## Output Requirements
Return in this order:
1) Implementation Plan (short, step list)
2) File-by-file diff style output (or clearly separated file contents)
3) Tests added/updated
4) Migration(s), if any
5) Notes: risks, follow-ups, and any assumptions

---

## INPUT A — `design_doc.md`
<PASTE CONTENT>

## INPUT B — `agents_codex.md`
<PASTE CONTENT>

## INPUT C — Jira Ticket (LOCKED)
<PASTE LOCKED TICKET>
