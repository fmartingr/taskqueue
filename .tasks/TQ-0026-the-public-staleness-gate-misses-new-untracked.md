---
id: TQ-0026
title: The public/ staleness gate misses new untracked files
status: done
priority: normal
labels:
  - bug
  - component/ci
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T18:27:58+02:00
---

## Finding

`git diff --exit-code -- public` only detects modifications to already-tracked files, so a new uncommitted build output is invisible to the 'public/ is not stale' gate.

Source: `ci.yml:68`

## How it fails

VERIFIED in a scratch repo: with `public/index.html` committed, an untracked `public/logo.svg` still leaves `git diff --exit-code -- public` at exit 0. Add an asset to frontend/ and to build.ts's COPIES list but forget to `git add public/<newfile>` (it is generated output, easy to overlook): CI regenerates it locally, sees a clean diff, goes green — while the committed tree and the release binary lack the file. It also would not be in embed.go's explicit list, so the binary 404s for it at runtime with no build error. `git status --porcelain -- public` catches both.

## Suggested fix

Use `git status --porcelain -- public` instead of `git diff --exit-code -- public`, which only sees already-tracked files.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T18:27:58+02:00 — Fixed as part of TQ-0036, which rewrote both workflows and warned against re-introducing this. The gate is now git status --porcelain -- public, which sees untracked build output where git diff --exit-code did not, and it prints what is stale before failing.
