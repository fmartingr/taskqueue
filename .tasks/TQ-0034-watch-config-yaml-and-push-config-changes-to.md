---
id: TQ-0034
title: Watch config.yaml and push config changes to the board
status: todo
priority: normal
labels:
  - config
  - api
  - frontend
depends_on:
  - TQ-0029
  - TQ-0033
created: 2026-08-25T12:07:30+02:00
updated: 2026-08-25T12:07:30+02:00
---

## Proposal

Once the project config exists (TQ-0029) the board fetches it once at load, so
editing `.tasks/config.yaml` — adding a label, changing a colour, adding a
severity — only shows up after a manual reload. Extend the change detection from
the event-stream ticket to cover the config file and push a `config` event; the
board refetches `GET /api/config` and re-renders chips, selects and filters.

## Details

- The config is one more file in the same fingerprint check; no second
  mechanism, no watcher of its own.
- The board keeps the configuration as the raw JSON it fetched and derives label
  colours, display names and severity options from it at render time, so there
  is no second source of truth to keep in sync.
- **A half-saved file must not take the board down.** An editor writing
  `config.yaml` will briefly leave it invalid. Decide the behaviour: the
  recommendation is that `GET /api/config` returns the parse error in the
  standard error shape, the stream pushes an `error` event, and the board shows
  a toast while continuing with the configuration it already has — so a
  mid-save file does not blank the labels.
- The CLI needs nothing here: it reads the config per command already.
- Removing or renaming a label that tasks still carry is the tolerate-on-read
  case from TQ-0030. A config event must not make existing chips disappear.

## Acceptance criteria

- Editing `.tasks/config.yaml` updates an open board within a second: new labels
  appear in the filter bar, colours change, severity options update.
- An invalid config surfaces an error and leaves the board usable with the last
  good configuration.
- Reverting the file restores the board with no reload.
- Tests: a config change emits an event; a malformed config emits an error and
  the last good config stays available.
