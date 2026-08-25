---
id: TQ-0028
title: A 1 MB PNG serves as the favicon and ships in every binary
status: todo
priority: low
labels:
  - review
  - frontend
  - build
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

build.ts copies frontend/icon.png verbatim with no downscaling, so a 1254x1254, 1,047,994-byte PNG serves as the browser tab favicon and is embedded into every release binary.

Source: `build.ts:15`

## How it fails

VERIFIED: `curl http://127.0.0.1:7999/icon.png` returns 1,047,994 bytes, and index.html:7 references it as `<link rel="icon" type="image/png" href="/icon.png">` — every board page load pulls a megabyte to draw a 16x16 tab icon. The same megabyte is go:embed'ed into each of the four release archives, roughly 10% of the 10.8 MB binary, purely for a favicon.

## Suggested fix

Downscale the icon for the favicon (256px is plenty) and keep the full-size artwork for the README.

Filed from a `/code-review` pass at max effort.
