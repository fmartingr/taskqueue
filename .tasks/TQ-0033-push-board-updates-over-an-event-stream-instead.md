---
id: TQ-0033
title: Push board updates over an event stream instead of polling
status: done
priority: normal
labels:
  - component/api
  - component/frontend
  - feature
created: 2026-08-25T12:07:30+02:00
updated: 2026-08-26T14:00:20+02:00
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

---

## Notes

- 2026-08-25T18:42:55+02:00 — The change fingerprint has to cover .taskqueue.yaml as well as the task directory (TQ-0034 builds on it), so the marker's own edits reach the board.
- 2026-08-26T14:00:20+02:00 — Done. SSE at GET /api/events, a ticker-driven fingerprint on the server, and the poll kept as the fallback.

  Measured: ~500 ms from 'tq add' in a terminal to the board being told, worst case 501 ms. That is why the tick is 500 ms rather than the ~1s the ticket suggested — at one second I measured 1001 ms worst case, which fails 'well under a second' on the nose. The scan it doubles is a readdir and a stat per file, and it only runs while a board is connected.

  Two bugs the layering caught that the layer below could not:

  - RequestLogger wraps the ResponseWriter to record the status, and embedding http.ResponseWriter does not carry http.Flusher across. /api/events returned 'this server cannot stream' — but only behind the logger, which the API tests do not use, so all seven new Go tests passed against a broken server. The browser layer found it immediately. There is now a test that streams through the logger.
  - browser/poll.test.ts had stopped testing the poll: all five passed on the new stream and would have kept passing if the poll were deleted. They now open with /api/events refused, and went from instant to 31s, which is the proof they are on the fallback path.

  Design notes worth keeping:

  - The server's error frame is called 'scan-failed', not 'error'. EventSource dispatches its own connection failures to a listener named 'error', so a server frame by that name arrives indistinguishable from the stream dropping.
  - The keep-alive is a named 'ping' event rather than an SSE comment. A comment keeps the connection open but EventSource discards it without dispatching, so the page cannot use it to notice a half-open connection.
  - A change arriving while the user is mid-drag or has a dialog open is held and applied when they finish, not dropped. The poll can drop — another turn is three seconds away — but the stream has no next turn.
  - Deviated from one requirement: rather than dropping a slow client, each gets a one-slot mailbox with a non-blocking send. An event is a signal to refetch, not a record, so the pending one already covers the new one. Bounded memory, a tick that never blocks, and no board kicked off.

  The review at high effort found ten things, all fixed. The three that mattered: the fallback poll was fully suppressed while streaming, so a single failed fetch left the board stale forever — nothing retries, because the stream only speaks when the fingerprint changes; a subscribe landing after the hub stopped would hold http.Server.Shutdown for its full timeout, reachable in practice because the board reconnects every 500 ms; and scan-failed was deduplicated per-hub, so reloading the page after the directory broke told the new board nothing. Also: overlapping refreshes could install an older listing, the footer flashed 'polling' on every load (streaming is tri-state now), and a newline in a path would have split an SSE frame.
