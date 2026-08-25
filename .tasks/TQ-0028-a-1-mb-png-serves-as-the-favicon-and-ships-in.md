---
id: TQ-0028
title: A 1 MB PNG serves as the favicon and ships in every binary
status: done
priority: low
labels:
  - component/build
  - performance
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T18:45:50+02:00
---

## Finding

build.ts copies frontend/icon.png verbatim with no downscaling, so a 1254x1254, 1,047,994-byte PNG serves as the browser tab favicon and is embedded into every release binary.

Source: `build.ts:15`

## How it fails

VERIFIED: `curl http://127.0.0.1:7999/icon.png` returns 1,047,994 bytes, and index.html:7 references it as `<link rel="icon" type="image/png" href="/icon.png">` — every board page load pulls a megabyte to draw a 16x16 tab icon. The same megabyte is go:embed'ed into each of the four release archives, roughly 10% of the 10.8 MB binary, purely for a favicon.

## Suggested fix

Downscale the icon for the favicon (256px is plenty) and keep the full-size artwork for the README.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T18:45:50+02:00 — frontend/favicon.png is a new 256x256 derivative of the full-size artwork, 12,734 bytes against 1,047,994. build.ts copies it in place of icon.png, index.html and the go:embed list point at it, and public/icon.png is gone.
- 2026-08-25T18:45:50+02:00 — Generated with ImageMagick at 256 colours, not just resized. Plain downscaling gave 50,841 bytes; the palette gives 12,734 for line art. Measured the cost rather than assuming: RMSE 3.8 percent, 1.2 percent of pixels differ, invisible at the size a browser draws a tab icon.
- 2026-08-25T18:45:50+02:00 — Not a build step. Bun has no image pipeline and this project takes no JavaScript dependencies, so the file is generated once and committed, with the exact command in a comment above COPIES in build.ts for whoever changes the artwork.
- 2026-08-25T18:45:50+02:00 — Verified end to end: the binary drops from 10,788,002 to 9,731,234 bytes, GET /favicon.png returns 200 with 12,734 bytes, GET /icon.png returns 404, and the page links the new path. frontend/icon.png stays for the README, which renders it at 256 wide.
