---
id: TQ-0078
title: Bind address from the project config
status: done
priority: normal
labels:
  - feature
  - component/config
  - component/api
created: 2026-08-26T12:52:30+02:00
updated: 2026-08-26T13:33:52+02:00
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

---

## Notes

- 2026-08-26T13:33:52+02:00 — Done. server.host and server.port in .taskqueue.yaml, sitting between the environment and the built-in default.

  The one design decision worth recording: Port is a *int, not an int. port: 0 is a real value meaning 'let the OS pick', which the browser and integration harnesses rely on, and a plain int cannot tell that from a project pinning nothing. Mutation-tested — treating 0 as absent fails two tests at two layers.

  Validation is at load, with the file named, because the alternative is a bind error that names neither the file nor the key. The review caught that this covered the port and not the host: 'host: [::1]' is the spelling people reach for, since that is how the address is written once a port is on it, and left alone it reached net.Listen as [[::1]]:7331 — and ExposedHost, unable to parse it, warned that a loopback address was reachable from the network. Both now refused at load, with the unbracketing in the message.

  A non-loopback host warns on start-up, on stderr so it cannot be mistaken for the banner a script parses the address out of.

  Two things the review changed about how this is tested. The exposure warning was being tested by actually running 'tq serve --host 0.0.0.0', which puts an unauthenticated board on every interface of whatever machine runs 'go test' — a firewall prompt locally, a real exposure on CI. It now drives the warning function directly, with the loopback half still covered through a real serve because loopback is safe to bind. And the serve tests raced on the shared output buffer: 'tq serve' writes from one goroutine while the test polls from another. Fixed with a mutex-guarded buffer, which also closes the pre-existing race in TestCLIServePrintsTheAddressItActuallyGot; go test -race is clean.

  Also moved signal.Notify above the banner in runServe. The banner is what tells a caller the server is up, so a signal arriving between the two met Go's default disposition and killed the process instead of shutting it down. That window was masked only because runServe never called signal.Stop, so an earlier test left a handler registered for the process lifetime.

  Left out deliberately: the bind address is not added to GET /api/config. The ticket floated it as an option rather than a requirement, the board does not need it, and the JSON there is a stable contract.
