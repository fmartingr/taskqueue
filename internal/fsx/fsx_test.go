package fsx_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fmartingr/taskqueue/internal/fsx"
)

// wantReadableByAll checks the mode WriteAtomic chmods its staging file to,
// which is what keeps a generated document readable by more than the account
// that wrote it whatever umask that account runs under. Windows has no POSIX
// mode bits and Go reports 0666 for anything writable there, so there is
// nothing to check.
func wantReadableByAll(t *testing.T, info os.FileInfo) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("mode = %v, want -rw-r--r--", perm)
	}
}

// The documents WriteAtomic replaces are ones tq did not author — a project's
// marker, a guide beside its tasks — so the replacement has to arrive in one
// step: the name holds a different file afterwards, and a reader that opened
// the old one still reads the whole of it. A truncating write would keep the
// file and tear that read, and a run cut short would leave the document
// destroyed rather than merely stale.
//
// The fsync is not covered: whether the bytes reached the platter before the
// rename only shows up in a crash, and no assertion inside the test process can
// see it, so it is left untested on purpose rather than faked (TQ-0022).
func TestWriteAtomicReplacesTheFileRatherThanRewritingIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marker.yaml")
	was := []byte("version: 1\npath: .tasks\n")
	if err := os.WriteFile(path, was, 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	now := []byte("version: 1\npath: .tasks\ncolumns: [inbox, todo, done]\n")
	if err := fsx.WriteAtomic(path, now); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("the name still holds the file it held: the content was written into it rather than moved onto it")
	}
	wantReadableByAll(t, after)
	if got, err := os.ReadFile(path); err != nil || string(got) != string(now) {
		t.Errorf("file holds %q (err %v), want %q", got, err, now)
	}

	held, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(held) != string(was) {
		t.Errorf("a reader holding the file across the write read %q, want the old content %q", held, was)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "marker.yaml" {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Errorf("directory contains %v, want only marker.yaml: the staging file was left behind", got)
	}
}

func TestWriteAtomicCreatesTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	content := []byte("# Task queue\n")

	if err := fsx.WriteAtomic(path, content); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	wantReadableByAll(t, info)
	if got, err := os.ReadFile(path); err != nil || string(got) != string(content) {
		t.Errorf("file holds %q (err %v), want %q", got, err, content)
	}
}

// A write that fails takes its staging file with it. The content is already
// written and closed by the time the rename is attempted, so a failure there is
// the one place a half-finished write could leave a .tq-*.tmp beside the
// documents — and a directory at the destination is a rename that cannot
// succeed.
func TestWriteAtomicLeavesNothingBehindWhenItFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in-the-way")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := fsx.WriteAtomic(path, []byte("version: 1\n")); err == nil {
		t.Fatal("WriteAtomic over a directory should fail")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "in-the-way" {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Errorf("directory contains %v, want only in-the-way: the staging file was left behind", got)
	}
}
