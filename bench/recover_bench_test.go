package bench

// Benchmarks for the two-tier recovery path (src-gluon/recover.go). Their
// job is to quantify gluon#8 item 2 (per-call grammar conversion): every
// irregular line pays up to len(lineRules) ParseCSTWithOptions calls, each
// of which re-converts and re-compiles the grammar. When gluon grows a
// reusable Parser handle, these numbers should drop sharply — keep them as
// the before/after record.

import (
	"os"
	"path/filepath"
	"testing"

	robotsgluon "github.com/accretional/proto-robotstxt/src-gluon"
)

// BenchmarkRecoverMalformed runs the full two-tier parse over each
// malformed-tier corpus file (tier 2 engages: per-line StartRule parses +
// irregular fallback).
func BenchmarkRecoverMalformed(b *testing.B) {
	g, err := robotsgluon.Default()
	if err != nil {
		b.Fatalf("grammar: %v", err)
	}
	root, err := repoRoot()
	if err != nil {
		b.Skip(err)
	}
	files, _ := filepath.Glob(filepath.Join(root, "testdata", "malformed", "*.txt"))
	if len(files) == 0 {
		b.Skip("no malformed corpus")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec, err := g.Recover(data)
				if err != nil {
					b.Fatalf("Recover: %v", err)
				}
				if rec.Strict == nil && len(rec.Lines) == 0 {
					b.Fatal("no output")
				}
			}
		})
	}
}

// BenchmarkRecoverStrictPassthrough pins the tier-1 fast path: Recover on
// spec-valid input must cost the same as a plain strict parse.
func BenchmarkRecoverStrictPassthrough(b *testing.B) {
	g, err := robotsgluon.Default()
	if err != nil {
		b.Fatalf("grammar: %v", err)
	}
	root, err := repoRoot()
	if err != nil {
		b.Skip(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "testdata", "realistic-large.txt"))
	if err != nil {
		b.Skip("strict corpus file missing")
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rec, err := g.Recover(data)
		if err != nil || rec.Strict == nil {
			b.Fatalf("expected tier-1 success: %v", err)
		}
	}
}
