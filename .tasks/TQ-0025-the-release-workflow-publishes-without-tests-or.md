---
id: TQ-0025
title: The release workflow publishes without tests or a fresh public/
status: todo
priority: high
labels:
  - review
  - ci
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

The release workflow runs goreleaser with no preceding test, lint, or public/-freshness step, so a tag can publish binaries from a commit that never passed CI and that embeds a stale frontend.

Source: `release.yml:22`

## How it fails

ci.yml's frontend job carries the comment 'public/ is committed so releases need no Bun; this job fails if it is stale' — but it triggers only on `push: branches: [main]` and pull_request, never on `push: tags: ["v*"]`. Edit frontend/app.ts, forget `make frontend`, then `git commit && git tag v0.2.0 && git push --follow-tags`: CI and Release start in parallel with no `needs:` between them, so goreleaser publishes all four archives embedding the OLD public/app.js while the job meant to prevent exactly this is still running — and then fails, after the release is out.

## Suggested fix

Gate the release job on the same checks CI runs: tests, lint, and a public/ freshness check, before goreleaser publishes.

Filed from a `/code-review` pass at max effort.
