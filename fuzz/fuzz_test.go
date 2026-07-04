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
		last := int32(0)
		for _, e := range events {
			if e.Line <= 0 || e.Line < last {
				t.Fatalf("event lines not positive/ordered: %v\ninput: %q", events, data)
			}
			last = e.Line
		}
	})
}
