---
id: TQ-0078
title: Bind address from the project config
status: todo
priority: normal
labels:
  - feature
  - component/config
  - component/api
created: 2026-08-26T12:52:30+02:00
updated: 2026-08-26T12:52:30+02:00
---

## Proposal

Let a project pin the address `tq serve` binds to, in `.taskqueue.yaml`:

```yaml
version: 1
path: .tasks
server:
  port: 7412
```

Two projects served at once collide on the default 7331 today: the second
`tq serve` dies with "address already in use", and the way out is remembering
`--port` per project, every time. A committed port means each project has its
own, the same one for everyone who clones it, and the browser tab stays
bookmarkable.

A `server:` block rather than a bare `port:` because `host` is its immediate
neighbour and belongs in the same place. Both keys are optional; adding them is
additive, so no `version` bump.

## Precedence

Flag, then environment, then config, then built-in default — the existing rule
with one layer inserted:

```
--port 8080  >  TQ_PORT=8080  >  server.port in .taskqueue.yaml  >  7331
```

Same for host. `internal/cli/cli.go:738` resolves both today with
`envOr("TQ_PORT", defaultPort)` as the flag's default value, which is where the
config has to slot in. Each layer wants a test; the interesting one is that a
flag still beats a committed config.

## Details worth getting right

- **A taken port must still fail loudly.** Two projects can pick the same
  number, and silently falling back to another port would break the very thing
  this feature buys — the address being predictable. Keep "address already in
  use" and let it be a real error.
- **`port: 0` stays valid**, meaning "let the OS choose": the browser and
  integration harnesses rely on it, and the banner already prints the resolved
  address.
- **Validate the range** (0-65535) at load time with a message naming the file,
  rather than letting a nonsense value reach the listener.
- **`host` in a committed file is a safety question.** The server has no
  authentication, so a repository that commits `host: 0.0.0.0` exposes the board
  on the network of everyone who clones it and runs `tq serve`. Support the key,
  but say that plainly in the README, and consider a one-line warning on startup
  when the bound host is not a loopback address.
- `GET /api/config` already returns the config, so the board can show the port
  it was served from without new plumbing.

## Acceptance criteria

- `server.port` and `server.host` in `.taskqueue.yaml` change where `tq serve`
  binds, and the banner prints the resolved address as it does now.
- Precedence holds in all four layers, each covered by a test.
- An out-of-range or non-numeric port fails at load with the file named.
- `port: 0` still works, and the harnesses keep passing.
- README and `tq help` document the keys and the precedence, including the
  warning about a non-loopback host in a committed file.
