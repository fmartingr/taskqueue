---
id: TQ-0081
title: A dialog reopened within a millisecond of closing is swallowed
status: todo
priority: low
labels:
  - bug
  - component/frontend
created: 2026-08-26T13:12:58+02:00
updated: 2026-08-26T13:12:58+02:00
---

## Finding

`<dialog>` dispatches its `close` event asynchronously, one task after the
`open` attribute has already gone. The migrated dialogs *derive their existence*
from `openTaskID` / `creating` (`App.vue`), so a reopen landing inside that
window is destroyed by the late `close`: `openTaskID` is set to the new id, then
the queued handler nulls it. The dialog never opens and the click is swallowed.

The board this replaced was immune in kind — `openTask()` called `showModal()`
imperatively on every click, so a late `close` could only desync the poll's busy
flag, never eat the open. This one is specific to deriving mount state from a
ref.

## How it fails

Reproduces about 40% of the time via `page.focus()` + `keyboard.press()`
back-to-back, occasionally at a 1 ms delay, and never at 5 ms or more or with a
real mouse (12/12 clean). A human will not hit it. A browser test that closes a
dialog and reopens it without an intervening wait is a latent flake, which is
how it was found.

## Related shape, same file

`state.ts`'s poll gate reads `openTaskID`, but `TaskDialog` only mounts while
`tasks` still contains that id. If `openTaskID` ever names a task that is not in
the list, the dialog never mounts, its `close` never fires, and the poll is dead
for the life of the page. No trigger exists today — there is no DELETE endpoint,
and the poll stands down while the dialog is open — so this is a guard worth
adding, not a bug worth chasing.

## Suggested fix

Have `App.vue` ignore a `close` for a dialog it has already reopened — compare
the id the emitting dialog was mounted with — rather than blindly nulling. Do
not add a test at that timing granularity; the test is the flake.

Found by the review of TQ-0076.
