---
id: TQ-0034
title: Watch .taskqueue.yaml and push config changes to the board
status: done
priority: normal
labels:
  - component/api
  - component/config
  - feature
depends_on:
  - TQ-0029
  - TQ-0033
created: 2026-08-25T12:07:30+02:00
updated: 2026-08-26T16:50:54+02:00
---

## Proposal

Once the project config exists (TQ-0029) the board fetches it once at load, so
editing `.taskqueue.yaml` — adding a label, changing a colour, adding a
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

- Editing `.taskqueue.yaml` updates an open board within a second: new labels
  appear in the filter bar, colours change, severity options update.
- An invalid config surfaces an error and leaves the board usable with the last
  good configuration.
- Reverting the file restores the board with no reload.
- Tests: a config change emits an event; a malformed config emits an error and
  the last good config stays available.

---

## Notes

- 2026-08-26T16:50:50+02:00 — Done. The stream carries a second kind of frame.

  The fingerprint behind /api/events was already covering .taskqueue.yaml (TQ-0033
  added it for this ticket), but as one value with the task directory, so a marker
  edit arrived as a 'tasks' frame and sent the board to the wrong endpoint. It is
  two readings now — taskFingerprint and configFingerprint — pushed as 'tasks' and
  'config'. A stream opens with one of each, so a board that reconnected after
  missing a change refetches both.

  The stream error the proposal recommended is not there, and deliberately: the
  frames say only that the marker moved, the board refetches GET /api/config, and
  the failure it gets back is the one error path. One mechanism instead of two,
  and the message comes from the same place either way. GET /api/config now
  answers a broken marker with 500 invalid_config (config.ErrConfig mapped in
  writeStoreError) rather than the 400 validation_error it fell through to.

  configFingerprint stats, never parses — config.ConfigPath is FindConfig's walk
  without the load. Parsing here would make every mid-save state of a half-written
  file look identical, and the board would never be told when it settled.

  Two things the ticket did not call for, both found while writing it:
  - One buffered slot per subscriber could not hold two kinds of signal: a 'config'
    arriving behind a 'tasks' was dropped as a repeat. Subscribers now hold a
    pending set coalesced by name, woken by an empty channel.
  - A config change refetches the listing too. Store.List resolves each status
    against the project's columns and sorts by its priorities before serving, so
    changing the board moves the cards — but only when the config really changed,
    or every reconnect would cost two listings.

  The fallback poll reads the config as well now. Nothing else would notice a
  marker that changed while the stream is down, and the poll's whole promise is
  that the board is never silently stale.

  Tests: Go for both fingerprints and the frames (including a half-saved marker,
  and both kinds of change in one tick); config for ConfigPath answering where
  FindConfig errors; browser/config.test.ts for the four acceptance criteria —
  recolour and a new filter option, a column added, an unparsable file leaving the
  board on its last good configuration behind a toast, and reverting restoring it.
  Full suite green: go test, integration, frontend, browser (52), lint, build.
