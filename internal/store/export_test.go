package store

// The store's tests live in store_test, the external test package, because the
// fixtures they share with every other package are in tqtest — and tqtest
// imports the store, so an in-package test file cannot reach them. These two
// names are what those tests still need from the inside.

// RetireOldFile exposes retireOldFile to the external test package.
var RetireOldFile = retireOldFile

// Locate exposes locate to the external test package.
func (s *Store) Locate(id string) (string, error) { return s.locate(id) }
