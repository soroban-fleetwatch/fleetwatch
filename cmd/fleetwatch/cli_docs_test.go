package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	cliDocsBegin = "<!-- BEGIN GENERATED COMMAND REFERENCE: do not edit; run FLEETWATCH_UPDATE_DOCS=1 go test ./cmd/fleetwatch -run TestCLIReferenceUpToDate -->"
	cliDocsEnd   = "<!-- END GENERATED COMMAND REFERENCE -->"
)

// cliReference renders each command's synopsis from usage and its flags from
// the command's own -h output, so docs/CLI.md cannot drift from the code.
func cliReference(t *testing.T) string {
	t.Helper()
	synopsis := map[string]string{}
	for _, line := range strings.Split(usageText(), "\n") {
		trimmed := strings.TrimSpace(line)
		if name, _, ok := strings.Cut(trimmed, " "); ok && strings.HasPrefix(line, "  ") {
			synopsis[name] = trimmed
		}
	}
	var b strings.Builder
	for _, command := range documentedCommands {
		var out, errs bytes.Buffer
		if code := run([]string{command, "-h"}, &out, &errs); code != 0 {
			t.Fatalf("%s -h exit %d", command, code)
		}
		line, ok := synopsis[command]
		if !ok {
			t.Fatalf("usage has no synopsis for %s", command)
		}
		b.WriteString("### `" + command + "`\n\n```text\nfleetwatch " + line + "\n```\n\n```text\n")
		b.WriteString(strings.TrimRight(errs.String(), "\n") + "\n```\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func TestCLIReferenceUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "CLI.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	start := strings.Index(doc, cliDocsBegin)
	end := strings.Index(doc, cliDocsEnd)
	if start < 0 || end < start {
		t.Fatalf("%s lacks generated-reference markers", path)
	}
	want := doc[:start] + cliDocsBegin + "\n\n" + cliReference(t) + "\n" + doc[end:]
	if want == doc {
		return
	}
	if os.Getenv("FLEETWATCH_UPDATE_DOCS") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("%s is out of date; run FLEETWATCH_UPDATE_DOCS=1 go test ./cmd/fleetwatch -run TestCLIReferenceUpToDate", path)
}
