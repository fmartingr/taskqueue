package task

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

const fmDelimiter = "---"

// ErrInvalidTaskFile marks a file that exists but cannot be understood, which
// callers distinguish from "no such task" (the HTTP API reports it as a server
// error rather than a bad request).
var ErrInvalidTaskFile = errors.New("invalid task file")

// ParseTask reads one Markdown task file. The filename is only used to make
// errors actionable; the authoritative ID lives in the frontmatter.
func ParseTask(filename string, data []byte) (Task, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != fmDelimiter {
		return Task{}, fmt.Errorf("%w: %s: missing YAML frontmatter (file must start with %q)", ErrInvalidTaskFile, filename, fmDelimiter)
	}

	// Only an unindented delimiter closes the block. An indented one is data:
	// YAML writes a multi-line value as an indented block scalar, and a line
	// of it may well be "---". Trimming here would end the frontmatter inside
	// that value, and the renderer produces exactly such a file.
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == fmDelimiter {
			closing = i
			break
		}
	}
	if closing == -1 {
		return Task{}, fmt.Errorf("%w: %s: unterminated YAML frontmatter (missing closing %q)", ErrInvalidTaskFile, filename, fmDelimiter)
	}

	var task Task
	dec := yaml.NewDecoder(strings.NewReader(strings.Join(lines[1:closing], "\n")))
	dec.KnownFields(true) // unknown keys would be silently dropped on the next write
	// io.EOF means the frontmatter block was empty; validation reports the
	// missing fields, which is more useful than "EOF".
	if err := dec.Decode(&task); err != nil && !errors.Is(err, io.EOF) {
		return Task{}, fmt.Errorf("%w: %s: invalid YAML frontmatter: %v", ErrInvalidTaskFile, filename, err)
	}

	// Body is kept verbatim apart from the blank lines around it.
	task.Body = strings.Trim(strings.Join(lines[closing+1:], "\n"), "\n")

	if err := task.Validate(); err != nil {
		return Task{}, fmt.Errorf("%w: %s: %v", ErrInvalidTaskFile, filename, err)
	}
	return task, nil
}

// RenderTask produces the full Markdown file for a task. Rendering is stable:
// rendering a parsed task twice yields identical bytes, which keeps Git diffs
// limited to the fields that actually changed.
func RenderTask(t Task) ([]byte, error) {
	if err := t.ValidateForWrite(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString(fmDelimiter + "\n")

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(t); err != nil {
		return nil, fmt.Errorf("encode frontmatter for %s: %w", t.ID, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode frontmatter for %s: %w", t.ID, err)
	}

	buf.WriteString(fmDelimiter + "\n")
	if body := strings.Trim(t.Body, "\n"); body != "" {
		buf.WriteString("\n" + body + "\n")
	}
	return buf.Bytes(), nil
}
