---
id: TQ-0013
title: Any website can drive the local API
status: todo
priority: high
labels:
  - review
  - security
  - api
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

decodeJSON never checks Content-Type and no handler checks Origin, Sec-Fetch-Site or Host, while Decode is called once with no trailing-token check (line 266) — so any website the user visits can drive the local API with a plain HTML form, no JavaScript required.

Source: `server.go:264`

## How it fails

I verified against a live server: `curl -X POST -H 'Content-Type: text/plain;charset=UTF-8' -H 'Origin: https://evil.example' --data '{"title":"CSRF injected task"}' http://127.0.0.1:7799/api/tasks` returned 201 Created and the task landed on disk. text/plain is CORS-safelisted, so `<form action="http://127.0.0.1:7331/api/tasks" method=POST enctype="text/plain">` auto-submits with no preflight; the ignored trailing garbage is what makes it trivial, since a text/plain form emits `name=value` and only the field NAME needs to be the JSON. POST /api/tasks/{id}/notes is exploitable the same way. There is no Host check either, so a DNS-rebinding page gets full read access to every task plus the absolute task_dir from /api/status (server.go:215), and writeStoreError (server.go:255) echoes raw fs.PathError text like `open /Users/alice/work/secret-client-repo/.tasks: permission denied`.

## Suggested fix

Require `Content-Type: application/json` on mutating requests and reject cross-site `Origin`/`Sec-Fetch-Site`. Also reject trailing tokens after the JSON body.

Filed from a `/code-review` pass at max effort.
