// Package fuzz holds the fuzzing harnesses for the gluon robots.txt parser.
//
// Go-native fuzzing (go test -fuzz), seeded from the testdata corpus.
// Six harnesses (see fuzz/README.md for the full table): strict-parser
// invariants (FuzzParse), two-tier totality (FuzzRecover), byte-level and
// structure-aware differentials against the real google binaries
// (FuzzDifferential, FuzzStructured), decision parity (FuzzMatcher), and
// renderer identity (FuzzRenderRoundTrip). Remaining optional upgrade
// (docs/TODO.md item 2): a libFuzzer/C++ target with real
// libprotobuf-mutator for C++-side coverage feedback.
package fuzz

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

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
	f.Add([]byte("a \x0bb\ndisallow \x0c/x\n")) // \v/\f in the missing-colon path
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

// findDumpBin locates the robots_dump binary built by ./build.sh (or a raw
// bazel build); empty string if absent.
func findDumpBin() string {
	for _, p := range []string{
		"../gen/bin/robots_dump",
		"../bazel-bin/tools/robots-dump/robots_dump",
	} {
		if st, err := os.Stat(p); err == nil && st.Mode()&0o111 != 0 {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs
			}
		}
	}
	return ""
}

// FuzzDifferential is the phase-5 gate (docs/design/malformed-input.md):
// for ARBITRARY bytes, the two-tier parse must produce byte-identical
// events AND per-line metadata to google's parser. Any divergence on any
// input is a bug — either in our grammar/recovery or an undocumented
// google leniency to fold in. Runs the real C++ binary per execution
// (~200 execs/s); confirmed divergent inputs graduate into
// testdata/malformed/.
func FuzzDifferential(f *testing.F) {
	dump := findDumpBin()
	if dump == "" {
		f.Skip("robots_dump not built (run ./build.sh); skipping differential fuzz")
	}
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
		path := filepath.Join(t.TempDir(), "input.txt")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		google, err := robotsgluon.GoogleParse(dump, path)
		if err != nil {
			t.Fatalf("robots_dump on %q: %v", data, err)
		}
		if diffs := robotsgluon.DiffEvents(rec.Events, google.Events); len(diffs) != 0 {
			t.Fatalf("EVENT divergence on %q (%s):\n%s", data, rec.RecoverSummary(), strings.Join(diffs, "\n"))
		}
		if diffs := robotsgluon.DiffMetadata(rec.Metadata, google.Metadata); len(diffs) != 0 {
			t.Fatalf("METADATA divergence on %q (%s):\n%s", data, rec.RecoverSummary(), strings.Join(diffs, "\n"))
		}
	})
}

// findRobotsMain locates google's decision CLI; empty string if absent.
func findRobotsMain() string {
	for _, p := range []string{
		"../gen/bin/robots_main",
		"../bazel-bin/src-google/robots_main",
	} {
		if st, err := os.Stat(p); err == nil && st.Mode()&0o111 != 0 {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
		}
	}
	return ""
}

// FuzzMatcher differentially fuzzes the DECISION surface: for arbitrary
// (robots.txt bytes, agent, url) triples, our event-driven matcher port
// must reach the same allow/disallow verdict as google's robots_main.
func FuzzMatcher(f *testing.F) {
	robotsMain := findRobotsMain()
	if robotsMain == "" {
		f.Skip("robots_main not built (run ./build.sh); skipping matcher fuzz")
	}
	g, err := robotsgluon.Default()
	if err != nil {
		f.Fatalf("grammar: %v", err)
	}
	for _, pattern := range []string{"../testdata/*.txt", "../testdata/malformed/*.txt"} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil {
				f.Add(data, "FooBot", "https://example.com/x/y")
			}
		}
	}
	f.Add([]byte("User-agent: *\nDisallow: /\n"), "any", "https://e.com/")
	f.Add([]byte("User-agent: a\nAllow: /*.gif$\nDisallow: /\n"), "a", "https://e.com/p.gif")
	f.Fuzz(func(t *testing.T, robots []byte, agent, url string) {
		// robots_main receives agent/url as argv; NUL cannot cross exec.
		if strings.ContainsRune(agent, 0) || strings.ContainsRune(url, 0) {
			t.Skip()
		}
		ours, err := g.Allowed(robots, agent, url)
		if err != nil {
			t.Fatalf("Allowed must be total: %v", err)
		}
		path := filepath.Join(t.TempDir(), "robots.txt")
		if err := os.WriteFile(path, robots, 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(robotsMain, path, agent, url)
		gerr := cmd.Run()
		google := gerr == nil
		if gerr != nil {
			if ee, ok := gerr.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
				t.Fatalf("robots_main failed: %v", gerr)
			}
		}
		if ours != google {
			t.Fatalf("DECISION divergence: robots=%q agent=%q url=%q: gluon=%v google=%v",
				robots, agent, url, ours, google)
		}
	})
}

// FuzzStructured is the structure-aware differential fuzzer (docs/TODO.md
// item 2, cheap-first-step form): the fuzz input is interpreted as WIRE
// BYTES of a robotstxt.rep.Robotstxt message — so the mutator effectively
// mutates the typed rep (libprotobuf-mutator's core trick) — which is then
// RENDERED to robots.txt text (raw mode: adversarial reps produce
// adversarial text) and put through the same recovery-vs-google
// differential as FuzzDifferential. Seeds are the marshaled reps of the
// strict corpus, so mutation starts from deep, valid grammar shapes.
func FuzzStructured(f *testing.F) {
	dump := findDumpBin()
	if dump == "" {
		f.Skip("robots_dump not built (run ./build.sh); skipping structured fuzz")
	}
	g, err := robotsgluon.Default()
	if err != nil {
		f.Fatalf("grammar: %v", err)
	}
	files, _ := filepath.Glob("../testdata/*.txt")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		rep, err := g.Rep(data)
		if err != nil {
			continue
		}
		wire, err := proto.Marshal(rep)
		if err != nil {
			continue
		}
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, wire []byte) {
		msg, err := robotsgluon.NewRepMessage("Robotstxt")
		if err != nil {
			t.Fatal(err)
		}
		if err := proto.Unmarshal(wire, msg); err != nil {
			t.Skip() // not a decodable rep — mutation landed outside the schema
		}
		text, err := robotsgluon.RenderRep(msg, robotsgluon.RenderOptions{})
		if err != nil {
			t.Skip() // unrenderable shape (e.g. empty oneof arm)
		}
		rec, err := g.Recover(text)
		if err != nil {
			t.Fatalf("Recover must be total on rendered text %q: %v", text, err)
		}
		path := filepath.Join(t.TempDir(), "robots.txt")
		if err := os.WriteFile(path, text, 0o644); err != nil {
			t.Fatal(err)
		}
		google, err := robotsgluon.GoogleParse(dump, path)
		if err != nil {
			t.Fatalf("robots_dump on rendered %q: %v", text, err)
		}
		if diffs := robotsgluon.DiffEvents(rec.Events, google.Events); len(diffs) != 0 {
			t.Fatalf("EVENT divergence on rendered %q:\n%s", text, strings.Join(diffs, "\n"))
		}
		if diffs := robotsgluon.DiffMetadata(rec.Metadata, google.Metadata); len(diffs) != 0 {
			t.Fatalf("METADATA divergence on rendered %q:\n%s", text, strings.Join(diffs, "\n"))
		}
	})
}

// FuzzRenderRoundTrip pins render.go guarantee 1 beyond the corpus. A
// mutated rep may hold shapes no parse produces (e.g. a top-level directive
// AFTER a group — reparsing folds it into the group), so the invariant
// starts one parse in: text from a validating render must parse strictly,
// and from that point parse∘render is the identity on reps.
func FuzzRenderRoundTrip(f *testing.F) {
	g, err := robotsgluon.Default()
	if err != nil {
		f.Fatalf("grammar: %v", err)
	}
	files, _ := filepath.Glob("../testdata/*.txt")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if rep, err := g.Rep(data); err == nil {
			if wire, err := proto.Marshal(rep); err == nil {
				f.Add(wire)
			}
		}
	}
	f.Fuzz(func(t *testing.T, wire []byte) {
		msg, err := robotsgluon.NewRepMessage("Robotstxt")
		if err != nil {
			t.Fatal(err)
		}
		if err := proto.Unmarshal(wire, msg); err != nil {
			t.Skip()
		}
		text, err := robotsgluon.RenderRep(msg, robotsgluon.RenderOptions{Validate: true})
		if err != nil {
			t.Skip() // validation rejected the shape — allowed
		}
		rep2, err := g.Rep(text)
		if err != nil {
			t.Fatalf("validated render failed strict reparse: %v\ntext: %q", err, text)
		}
		text2, err := robotsgluon.RenderRep(rep2, robotsgluon.RenderOptions{Validate: true})
		if err != nil {
			t.Fatalf("re-render of parser-produced rep failed: %v", err)
		}
		rep3, err := g.Rep(text2)
		if err != nil {
			t.Fatalf("re-reparse failed: %v\ntext2: %q", err, text2)
		}
		if !proto.Equal(rep2, rep3) {
			t.Fatalf("parse-render not identity on parser-produced rep:\nrep2: %v\nrep3: %v\ntext2: %q", rep2, rep3, text2)
		}
	})
}
