---
id: TQ-0018
title: The -- terminator only protects the first argument
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T21:34:38+02:00
---

## Finding

The `--` end-of-flags terminator protects only the FIRST argument after it, because parse re-feeds `rest[1:]` into fs.Parse (`args = rest[1:]`), which re-enables flag parsing on every subsequent iteration.

Source: `cli.go:550`

## How it fails

I verified both directions. `tq note -- TQ-0001 "-1 failing"` — the form the contract at cli.go:97-98 and .tasks/AGENTS.md literally describe ("everything after it is an argument, even if it starts with -") — dies with `flag provided but not defined: -1 failing`, dumps usage and exits 1; only the reversed `tq note TQ-0001 -- "-1 failing"` works. The inverse leaks too: `tq add -- "-weird" --json` printed JSON and created the task, so `--json` written after `--` was still honoured as a flag instead of failing as a second positional. Any agent that follows the documented escaping rule for a note starting with `-` gets exit 1 and an unhelpful hint.

## Suggested fix

Once `--` is seen, treat every remaining argument as positional instead of re-entering flag parsing on the next iteration.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T21:34:38+02:00 — Fixed while adding the integration coverage in TQ-0073, which is the layer that proves it. parse now splits everything after -- off before parsing, instead of re-feeding what flag.Parse returns, which re-enabled flag parsing on every later argument.
- 2026-08-25T21:34:38+02:00 — Both directions verified against the real binary: tq note -- TQ-0001 '-1 test still failing' now exits 0 and keeps the note, and tq add -- '-weird title' --json now fails as two positionals rather than honouring --json.
