---
id: TQ-0081
title: A dialog reopened within a millisecond of closing is swallowed
status: todo
priority: low
labels:
  - bug
  - component/frontend
created: 2026-08-26T13:12:58+02:00
updated: 2026-08-26T23:58:34+02:00
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

---

## Notes

- 2026-08-26T18:01:38+02:00 — Revalidated on main (2d8ddfa): still present, and the suggested fix alone is not sufficient.

  Reproduced in a real Chromium against the real tq serve binary. The synthetic same-task reproduction fails 100% of the time for both the task dialog and the create dialog; a realistic Escape-then-Enter reopen was swallowed 8 times in 20 (40%), matching the rate in the original finding. A control reopen with a wait passes.

  A DOM trace pins the ordering and shows the defect has two halves, not one:

      right after close():                 dialog=mounted open=false id=TQ-0001
      right after click on other card:     dialog=mounted open=false id=TQ-0001
      after microtasks (Vue has flushed):  dialog=mounted open=false id=TQ-0002
      << close event fires >>
      after one macrotask:                 dialog=absent

  Vue's microtask flush beats the queued close, so the reopen is already lost before the late close arrives. Because v-if carries no :key (App.vue:37), the reopened dialog is patched in place rather than remounted, so onMounted(() => dialog.value?.showModal()) (TaskDialog.vue:54) never re-runs. The late close then nulls openTaskID and unmounts it.

  With only the stale-close id guard added, the dialog would survive as a mounted element with no open attribute and the click would still be swallowed — silently, which is worse. A complete fix needs both: the id comparison on close, AND a path that re-shows the element. A :key on the task id handles the different-task case; the same-task case needs the element re-shown explicitly.

  The related shape is also still unguarded: busy reads openTaskID (state.ts:113-119) while TaskDialog only mounts when tasks still contains that id (App.vue:19), with no reconciliation between them.
- 2026-08-26T23:58:34+02:00 — Correction from a review pass (2026-08-26): this task's premise that no trigger exists for a stranded openTaskID is WRONG. A reachable one was identified.

  applySignals awaits fetchTasks(), and a card click during that await sets openTaskID synchronously. If the change that fired the event was the task's file going away, the resolving listing drops it, so `open` becomes undefined, TaskDialog unmounts WITHOUT firing @close, openTaskID stays set, busy stays true — and from then on stream signals queue forever and the fallback poll is dead for the life of the page.

  TaskDialog.append()'s own `await refresh()` is a second path to the same state.

  Reference: frontend/components/App.vue:19. Treat the guard as fixing a reachable bug, not as defensive polish.
