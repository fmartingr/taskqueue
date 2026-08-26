package store

// The store's tests live in store_test, the external test package, because the
// fixtures they share with every other package are in tqtest — and tqtest
// imports the store, so an in-package test file cannot reach them. These two
// names are what those tests still need from the inside.

// RetireOldFile exposes retireOldFile to the external test package.
var RetireOldFile = retireOldFile

// Locate exposes locate to the external test package.
func (s *Store) Locate(id string) (string, error) { return s.locate(id) }

// ListAttempts exposes the bound on List's rescan, so a test can exhaust it
// without hard-coding the number.
const ListAttempts = listAttempts

// DuringScan installs a hook that runs inside every one of List's read windows:
// after the directory has been read, before the files are. It is the only way
// to drive the race List retries for without depending on timing.
func (s *Store) DuringScan(fn func()) { s.duringScan = fn }
