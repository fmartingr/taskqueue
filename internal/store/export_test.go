package store

// The store's tests live in store_test, the external test package, because the
// fixtures they share with every other package are in tqtest — and tqtest
// imports the store, so an in-package test file cannot reach them. These names
// are what those tests still need from the inside.

// Locate exposes locate to the external test package.
func (s *Store) Locate(id string) (string, error) { return s.locate(id) }

// ListAttempts exposes the bound on List's rescan, so a test can exhaust it
// without hard-coding the number.
const ListAttempts = listAttempts

// DuringScan installs a hook that runs inside every one of List's read windows:
// after the directory has been read, before the files are. It is the only way
// to drive the race List retries for without depending on timing.
func (s *Store) DuringScan(fn func()) { s.duringScan = fn }

// UpdateAttempts exposes the bound on update's retry, so a test can exhaust it
// without hard-coding the number.
const UpdateAttempts = updateAttempts

// DuringUpdate installs a hook that runs inside every one of a save's move
// windows: after the task's file has been located, before it is moved. It is
// the only way to drive the race update retries for without depending on
// timing.
func (s *Store) DuringUpdate(fn func()) { s.duringUpdate = fn }
