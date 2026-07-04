// Package bench benchmarks the gluon-based EBNF robots.txt parser
// (src-gluon) against Google's hand-rolled C++ parser (src-google, via the
// gen/bin/robots_dump event-dump binary).
//
// Run from the repo root:
//
//	go test -bench . -benchmem -run '^$' ./bench/
//
// or use bench/bench.sh, which also records results to gen/bench-latest.txt
// and prints a comparison table. See bench/README.md for methodology and
// interpretation notes.
//
// All benchmarks skip gracefully (b.Skipf) when their inputs are missing --
// no corpus files in testdata/, grammar failing to load, or robots_dump not
// built yet -- so a partial checkout never hard-fails the suite.
package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	robotsgluon "github.com/accretional/proto-robotstxt/src-gluon"
)

// Package-level sinks keep benchmarked results alive so the compiler cannot
// optimize the calls away.
var (
	sinkAST    any
	sinkEvents []robotsgluon.Event
)

// repoRoot walks up from the current working directory (the bench/ package
// directory when run under `go test`) until it finds go.mod. This keeps the
// benchmarks working regardless of where `go test` is invoked from.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// corpusFile is one testdata/*.txt input, preloaded into memory so file I/O
// never lands inside a timed region.
type corpusFile struct {
	name string // sub-benchmark name: base name, ".txt" stripped
	path string // absolute path, for the subprocess benchmark
	data []byte
}

// loadCorpus globs testdata/*.txt under the repo root and reads each file.
// The corpus is curated separately; nothing here assumes specific contents.
func loadCorpus(b *testing.B) []corpusFile {
	b.Helper()
	root, err := repoRoot()
	if err != nil {
		b.Skipf("cannot locate repo root: %v", err)
	}
	pattern := filepath.Join(root, "testdata", "*.txt")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		b.Skipf("bad corpus glob %q: %v", pattern, err)
	}
	sort.Strings(paths)
	files := make([]corpusFile, 0, len(paths))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			b.Logf("skipping unreadable corpus file %s: %v", p, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(p), ".txt")
		name = strings.ReplaceAll(name, " ", "_")
		files = append(files, corpusFile{name: name, path: p, data: data})
	}
	if len(files) == 0 {
		b.Skipf("no corpus files matching %s", pattern)
	}
	return files
}

// The grammar is compiled once per `go test` process and shared by every
// benchmark, so grammar compilation never lands inside a timed region.
var (
	grammarOnce sync.Once
	grammarVal  *robotsgluon.Grammar
	grammarErr  error
)

func grammar(b *testing.B) *robotsgluon.Grammar {
	b.Helper()
	grammarOnce.Do(func() {
		grammarVal, grammarErr = robotsgluon.Default()
	})
	if grammarErr != nil {
		b.Skipf("robotsgluon.Default() failed (core package not ready?): %v", grammarErr)
	}
	return grammarVal
}

// BenchmarkGluonParse measures Grammar.Parse -- the full-CST path that
// materializes a complete gluonpb.ASTDescriptor for the input. This is
// expected to be the slowest path by a wide margin; the interesting signal
// is its cost relative to BenchmarkGluonEvents on the same inputs.
func BenchmarkGluonParse(b *testing.B) {
	g := grammar(b)
	for _, f := range loadCorpus(b) {
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ast, err := g.Parse(f.data)
				if err != nil {
					b.Fatalf("Parse(%s): %v", f.path, err)
				}
				sinkAST = ast
			}
		})
	}
}

// BenchmarkGluonEvents measures Grammar.Events -- the google-parser-
// equivalent event stream. This is the path meant to be semantically
// bijective with Google's parser, so it is the fair (in-process) comparison
// point against BenchmarkGoogleDump.
func BenchmarkGluonEvents(b *testing.B) {
	g := grammar(b)
	for _, f := range loadCorpus(b) {
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ev, err := g.Events(f.data)
				if err != nil {
					b.Fatalf("Events(%s): %v", f.path, err)
				}
				sinkEvents = ev
			}
		})
	}
}

// BenchmarkGoogleDump measures Google's C++ parser via the gen/bin/robots_dump
// subprocess.
//
// CAVEAT: every iteration pays fork/exec, pipe I/O, and output re-parsing on
// top of the actual C++ parse (which is typically microseconds). These numbers
// are a ceiling on Google's per-file cost, dominated by process-spawn
// overhead -- roughly constant per file -- not a measurement of the parser
// itself. Treat them as a sanity anchor, not a head-to-head figure.
//
// Skipped under -short and whenever the binary is missing (run ./build.sh).
func BenchmarkGoogleDump(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping subprocess benchmark in -short mode")
	}
	root, err := repoRoot()
	if err != nil {
		b.Skipf("cannot locate repo root: %v", err)
	}
	dumpBin := filepath.Join(root, "gen", "bin", "robots_dump")
	info, err := os.Stat(dumpBin)
	if err != nil {
		b.Skipf("robots_dump not built at %s (run ./build.sh): %v", dumpBin, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		b.Skipf("%s is not an executable file (run ./build.sh)", dumpBin)
	}
	for _, f := range loadCorpus(b) {
		b.Run(f.name, func(b *testing.B) {
			b.SetBytes(int64(len(f.data)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ev, err := robotsgluon.GoogleEvents(dumpBin, f.path)
				if err != nil {
					b.Fatalf("GoogleEvents(%s, %s): %v", dumpBin, f.path, err)
				}
				sinkEvents = ev
			}
		})
	}
}

// rulesPerGroup is the number of Allow/Disallow lines per synthetic group;
// with the one User-agent line per group, each full group is 10 lines.
const rulesPerGroup = 9

// alphaToken renders n in base-26 lowercase letters ("a", "b", ... "ba",
// "bb", ...). RFC 9309's product-token identifier is 1*("-"/A-Z/"_"/a-z) --
// digits are NOT allowed -- so synthetic user-agent names must be letters.
func alphaToken(n int) string {
	buf := []byte{}
	for {
		buf = append([]byte{byte('a' + n%26)}, buf...)
		n /= 26
		if n == 0 {
			return string(buf)
		}
	}
}

// syntheticRobots generates a well-formed in-memory robots.txt with exactly
// `lines` non-empty lines: repeated groups of one User-agent line followed by
// rulesPerGroup alternating Disallow/Allow rules (the final group is
// truncated if lines is not a multiple of the group size).
func syntheticRobots(lines int) []byte {
	var sb strings.Builder
	written := 0
	for group := 0; written < lines; group++ {
		fmt.Fprintf(&sb, "User-agent: synthetic-bot-%s\n", alphaToken(group))
		written++
		for r := 0; r < rulesPerGroup && written < lines; r++ {
			if r%2 == 0 {
				fmt.Fprintf(&sb, "Disallow: /group-%d/rule-%d/\n", group, r)
			} else {
				fmt.Fprintf(&sb, "Allow: /group-%d/rule-%d/index.html\n", group, r)
			}
			written++
		}
	}
	return []byte(sb.String())
}

// BenchmarkGluonEventsScaling measures how Grammar.Events scales with input
// size on synthetic, uniformly-shaped files of 100, 1000, and 10000 lines
// (10x steps). The scaling shape matters more than absolute numbers: linear
// growth in ns/op (flat MB/s) means no super-linear blowup in the EBNF
// machinery as inputs grow.
func BenchmarkGluonEventsScaling(b *testing.B) {
	g := grammar(b)
	for _, lines := range []int{100, 1000, 10000} {
		data := syntheticRobots(lines)
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ev, err := g.Events(data)
				if err != nil {
					b.Fatalf("Events(synthetic %d lines): %v", lines, err)
				}
				sinkEvents = ev
			}
		})
	}
}
