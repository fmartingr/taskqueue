---
id: TQ-0095
title: A zoned IPv6 host in the config breaks every command, not just serve
status: todo
priority: normal
labels:
  - bug
  - component/config
created: 2026-08-28T12:58:46+02:00
updated: 2026-08-28T12:58:46+02:00
---

## Finding

`validateServer` rejects a valid IPv6 address carrying a zone, and it runs
inside config loading — so the failure is not limited to the command that
serves.

`net.ParseIP` returns nil for a zoned address (`fe80::1%en0`), and
`hostPattern` allows neither `%` nor `:`. `net.Listen` accepts the address
perfectly well.

Source: `internal/config/server.go` (~:59-62; locate by symbol, line numbers
drift).

## How it fails

`validateServer` is called from `loadConfig`, so a committed marker with a
zoned host makes **every** command fail — `tq list`, `tq add`, `tq show` — not
just `tq serve`. A project whose config is otherwise fine becomes unusable.

## Suggested fix

Validate the host the way the thing that binds it does. Either accept what
`net.SplitHostPort` plus a zone-aware parse accepts, or stop validating the
server block during config load and validate it where the listener is created —
so a bad server address breaks `tq serve` and nothing else.

The second is the smaller behavioural change and matches the shape of the
problem: a field only `serve` reads should only be able to break `serve`.

## Acceptance

- A marker with `host: fe80::1%en0` does not break `tq list`.
- `tq serve` binds it, or fails with a message naming the address.
- A test covering a zoned IPv6 host, and one covering an address only `serve`
  can reject.

Found by a code review during TQ-0061; not independently reproduced —
**confirm the reproduction before fixing.**
