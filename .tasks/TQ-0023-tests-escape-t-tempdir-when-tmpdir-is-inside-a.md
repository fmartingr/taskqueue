---
id: TQ-0023
title: Tests escape t.TempDir() when TMPDIR is inside a git repository
status: todo
priority: high
labels:
  - review
  - tests
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

`root := t.TempDir()` is not an isolation barrier here: taskDirTarget/DiscoverTaskDir walk UP out of the temp dir looking for `.git` and `.tasks`, so whenever TMPDIR sits inside a git repository the tests bind to that repository's task directory.

Source: `store_test.go:14`

## How it fails

VERIFIED: with `TMPDIR=<repo>/tmp` (common in Bazel/Nix/containerised CI, and with `export TMPDIR=$PWD/tmp`), `go test ./...` walked past every t.TempDir() to the enclosing `.git`, wrote 20+ junk files into the repo's own committed `.tasks/`, clobbered `.tasks/AGENTS.md`, and deleted the real task file there. TestDiscoverTaskDirNotFound (line 448) and TestOpenStoreCreatesTaskDirOnDemand (line 57) also fail. Separate escape route from the TQ_DIR one and needs a separate fix (e.g. `os.MkdirAll(root/".git")` in the helper).

## Suggested fix

Give each test root its own `.git` marker so the upward walk stops there, independently of where TMPDIR points.

Filed from a `/code-review` pass at max effort.
