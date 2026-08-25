---
id: TQ-0022
title: The atomicity test passes with atomicity removed
status: todo
priority: high
labels:
  - tests
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

TestUpdateRewritesFileAtomically verifies neither guarantee its name claims — not atomicity, not the updated-timestamp refresh; both assertions pass on deliberately broken code.

Source: `store_test.go:265`

## How it fails

VERIFIED by mutation, twice. (a) Replacing Store.write's CreateTemp/Chmod/Sync/Rename with a plain `os.WriteFile` — removing atomicity entirely, a headline guarantee in README:188 — leaves the whole suite green, because the only check is that the directory ends up holding one correctly-named file. (b) Deleting `task.Updated = time.Now().Truncate(time.Second)` from store.go:327 also leaves the suite green: the assertion is `updated.Updated.Before(updated.Created)`, and since both truncate to the second the values are equal, so `Before` is false whether or not the field is ever refreshed. This is why `go test` stayed green through all 15 findings.

## Suggested fix

Assert what the name claims: no temp file survives, the content is complete, and the updated timestamp actually moves (inject the clock or compare against a stored value rather than a same-second truncation).

Filed from a `/code-review` pass at max effort.
