// Package fuzz holds the fuzzing harnesses for the gluon robots.txt parser.
//
// Current harness: Go-native fuzzing (go test -fuzz), seeded from the
// testdata corpus. Planned (docs/TODO.md): structure-aware differential
// fuzzing with google/libprotobuf-mutator + libfuzzer, mutating
// robotstxt.rep messages (proto/rep.proto), rendering them to robots.txt
// text, and diffing the two parsers' deserializations.
package fuzz

import (
	"os"
	"path/filepath"
	"testing"

	robotsgluon "github.com/accretional/proto-robotstxt/src-gluon"
)

func addSeeds(f *testing.F) {
	f.Helper()
	// Both tiers: strict files must parse; malformed files exercise the
	// rejection paths. Fuzzing cares about "no panic/hang", not "parses".
	for _, pattern := range []string{"../testdata/*.txt", "../testdata/malformed/*.txt"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil {
				f.Add(data)
			}
		}
	}
	f.Add([]byte("User-agent: *\nDisallow: /\n"))
	f.Add([]byte("\xEF\xBB\xBFUser-agent: *\r\nAllow: /a%2fb # c\r\n"))
	f.Add([]byte(""))
}

// FuzzParse asserts the strict parser never panics or hangs, and that on
// every accepted input the whole derived pipeline holds together:
// CST -> typed rep and CST -> events must both succeed, and event line
// numbers must be positive and non-decreasing (they are emitted in source
// order).
func FuzzParse(f *testing.F) {
	addSeeds(f)
	g, err := robotsgluon.Default()
	if err != nil {
		f.Fatalf("grammar: %v", err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		ast, err := g.Parse(data)
		if err != nil {
			return // rejected by the strict grammar — fine
		}
		if _, err := robotsgluon.CSTToRep(ast); err != nil {
			t.Fatalf("accepted input failed CST->rep: %v\ninput: %q", err, data)
		}
		events, err := g.Events(data)
		if err != nil {
			t.Fatalf("accepted input failed CST->events: %v\ninput: %q", err, data)
		}
		assertEventOrder(t, events, data)
	})
}

// FuzzRecover asserts the two-tier parse is total: every byte sequence
// yields a Recovered without error or panic; events stay ordered; and when
// the strict tier accepted the input, recovery's events are byte-identical
// to the strict pipeline's (tier 2 must never shadow tier 1).
func FuzzRecover(f *testing.F) {
	addSeeds(f)
	g, err := robotsgluon.Default()
	if err != nil {
		f.Fatalf("grammar: %v", err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		rec, err := g.Recover(data)
		if err != nil {
			t.Fatalf("Recover must be total, failed on %q: %v", data, err)
		}
		assertEventOrder(t, rec.Events, data)
		// Metadata mirrors google's per-line accounting: at least the EOF
		// record, consecutively numbered from 1.
		if len(rec.Metadata) == 0 {
			t.Fatalf("no metadata records for %q", data)
		}
		for i, m := range rec.Metadata {
			if m.Line != int32(i+1) {
				t.Fatalf("metadata line numbers not consecutive at %d: %v (input %q)", i, m, data)
			}
		}
		if rec.Strict != nil {
			strict, err := g.Events(data)
			if err != nil {
				t.Fatalf("tier-1 success but Events failed on %q: %v", data, err)
			}
			if diffs := robotsgluon.DiffEvents(rec.Events, strict); len(diffs) != 0 {
				t.Fatalf("recover-vs-strict divergence on %q: %v", data, diffs)
			}
		}
	})
}

func assertEventOrder(t *testing.T, events []robotsgluon.Event, data []byte) {
	t.Helper()
	last := int32(0)
	for _, e := range events {
		if e.Line <= 0 || e.Line < last {
			t.Fatalf("event lines not positive/ordered: %v\ninput: %q", events, data)
		}
		last = e.Line
	}
}
