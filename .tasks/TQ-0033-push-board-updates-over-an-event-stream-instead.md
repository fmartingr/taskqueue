---
id: TQ-0033
title: Push board updates over an event stream instead of polling
status: todo
priority: normal
labels:
  - api
  - frontend
  - ux
created: 2026-08-25T12:07:30+02:00
updated: 2026-08-25T12:07:30+02:00
---

## How the board updates today

`frontend/app.ts:764` runs a 3 second `setInterval` that calls
`GET /api/tasks`, compares the serialized response against the previous one
(`state.lastPayload`) and re-renders only when it differs. The poll is skipped
while a card is being dragged, while either dialog is open and while the
quick-add composer is open, so an agent's change lands up to ~3 seconds late, or
indefinitely later if a dialog is sitting open. That interval is the entire
update mechanism — there is no push of any kind.

## Proposal

The server holds the connected boards, notices when the task directory changes,
and pushes a signal. The board refetches and re-renders. Polling stays as a
fallback for when the stream is unavailable.

## Transport: WebSocket or SSE — decide first

The request was for a WebSocket. Worth weighing before building:

- Nothing here needs client to server messages; every write already goes through
  REST. WebSocket's bidirectionality buys nothing yet.
- Go's standard library has no WebSocket implementation, so it means a
  dependency (`coder/websocket` is the maintained one) or hand-rolled RFC 6455
  framing. The repository has exactly one dependency today, `yaml.v3`, and
  AGENTS.md says to prefer the standard library.
- Server-sent events are `text/event-stream` plus `http.Flusher`: pure stdlib,
  one-way, and the browser's `EventSource` reconnects on its own.

Recommendation: SSE at `GET /api/events` for v1, and revisit WebSocket the day
the board needs to send something (presence, optimistic locking, live editing).
Everything below is identical either way — only the transport changes.

## How the server learns about a change

CLI writes never touch the server; that is the property that makes tq work. So
events cannot be emitted from the HTTP handlers alone.

- **Filesystem watcher** (fsnotify): another dependency, and AGENTS.md currently
  forbids watchers outright.
- **One server-side scan on a ticker** (~1s): fingerprint the directory (names,
  sizes, modtimes) and push when the fingerprint changes. No dependency, no
  cache, still "every read hits the disk" — and it is one scan for N boards
  instead of N browsers each scanning.

Recommendation: the ticker. Watch out for same-second rewrites that keep the
same size: `ModTime` has nanosecond precision on Linux and macOS but not on
every filesystem, so if the fingerprint proves unreliable, hash the file
contents — these directories are small.

## Event contract

- `tasks` — a signal carrying the new fingerprint, no task payload. The client
  refetches `GET /api/tasks`. One serialization path, a tiny stream, and the
  board already re-renders wholesale.
- Alternative: ship the full list in the event to save a round trip, at the cost
  of a second serialization path that can drift from the REST one.
- Leave room for `config` (its own ticket) and for `error`, so a directory that
  cannot be read reaches the existing error banner instead of going quiet.

## Frontend

- Connect on load; on `tasks`, refetch and re-render.
- Honour the guards that pause polling today (drag in progress, dialog open,
  composer open) by **queueing** the refresh and applying it when the guard
  clears, rather than dropping it as the poll does.
- Reconnect with backoff, and fall back to the 3 second poll while
  disconnected so the board is never silently stale.
- Surface connection state quietly; the status bar already exists.

## Requirements

- `tq serve` with no clients does nothing beyond the ticker.
- A client that disappears must not leak a goroutine or block the ticker.
- A slow client gets a bounded buffer and is dropped rather than growing memory.
- Graceful shutdown closes open streams; the signal handler already exists.
- Tests: a real streaming read against `httptest`, an event after a file
  changes, a disconnect that frees its goroutine, and shutdown that ends the
  stream.

## Architecture note

AGENTS.md says "Do not add a database, an index file, a cache or a filesystem
watcher", the PoC plan lists WebSockets and SSE as non-goals, and "polling
rather than realtime push" is one of its accepted limitations. This ticket
reverses that deliberately, so update AGENTS.md and the README in the same
change and keep the rules honest.

## Acceptance criteria

- `tq add` from a terminal shows up on an open board in well under a second,
  with polling disabled.
- With the stream unavailable the board still updates on the fallback poll.
- Repeated connect/disconnect shows no goroutine or memory growth (test).
- Docs and architecture rules updated in the same change.
