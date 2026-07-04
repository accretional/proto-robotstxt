package robotsgluon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractIrregular(t *testing.T) {
	cases := []struct{ name, line, key, value, reason string }{
		{"missing colon", "Disallow /tmp", "Disallow", "/tmp", "missing-colon-separator"},
		{"missing colon tabs", "Disallow\t\t/tmp", "Disallow", "/tmp", "missing-colon-separator"},
		{"junk multi token", "this is not a directive", "", "", "no-separator-multi-token"},
		{"single token", "junk", "", "", "no-separator"},
		{"empty key", ": value", "", "", "empty-key"},
		{"empty", "   ", "", "", "empty"},
		{"comment only", "  # hi", "", "", "comment-only"},
		{"ua with version", "User-agent: Example Bot/1.0", "User-agent", "Example Bot/1.0", "directive-outside-grammar"},
		{"key with space", "user agent: FooBot", "user agent", "FooBot", "directive-outside-grammar"},
		{"comment strip", "Disallow: /a #b", "Disallow", "/a", "directive-outside-grammar"},
		{"nul truncates", "Disallow: /x\x00#hidden", "Disallow", "/x", "directive-outside-grammar"},
		{"nul kills separator", "Disallow\x00: /x", "", "", "no-separator"},
		{"colon wins over ws", "user agent: FooBot", "user agent", "FooBot", "directive-outside-grammar"},
	}
	for _, c := range cases {
		key, value, reason := extractIrregular(c.line)
		if key != c.key || value != c.value || reason != c.reason {
			t.Errorf("%s: extractIrregular(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.name, c.line, key, value, reason, c.key, c.value, c.reason)
		}
	}
}

func TestSplitPhysicalLines(t *testing.T) {
	segs := splitPhysicalLines([]byte("a\nb\r\nc\rd\n"))
	want := []struct {
		num  int32
		text string
	}{{1, "a"}, {2, "b"}, {3, "c"}, {4, "d"}}
	if len(segs) != len(want) {
		t.Fatalf("got %d segments, want %d: %+v", len(segs), len(want), segs)
	}
	for i, w := range want {
		if segs[i].num != w.num || segs[i].text != w.text {
			t.Errorf("seg %d = {%d,%q}, want {%d,%q}", i, segs[i].num, segs[i].text, w.num, w.text)
		}
	}
}

// TestRecoverStrictUsesTier1 pins that spec-valid input takes the strict
// path and produces the same events as Events().
func TestRecoverStrictUsesTier1(t *testing.T) {
	g := mustGrammar(t)
	src := []byte("User-agent: *\nDisallow: /private\nSitemap: https://x.example/s.xml\n")
	rec, err := g.Recover(src)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Strict == nil {
		t.Fatal("tier 1 should have handled spec-valid input")
	}
	if rec.Lines != nil {
		t.Errorf("strict path should not produce line records, got %v", rec.Lines)
	}
	strict, err := g.Events(src)
	if err != nil {
		t.Fatal(err)
	}
	if diffs := DiffEvents(rec.Events, strict); len(diffs) != 0 {
		t.Errorf("recover-vs-strict events differ: %v", diffs)
	}
}

// TestRecoverGolden exercises tier 2 on a document mixing strict lines,
// google-tolerated deviations, and junk.
func TestRecoverGolden(t *testing.T) {
	g := mustGrammar(t)
	src := "User-agent: *\n" + // 1: strict startgroupline
		"Disallow /tmp\n" + // 2: missing colon -> DISALLOW
		"useragent: FooBot\n" + // 3: UA typo -> USER_AGENT
		"User-agent: Example Bot/1.0\n" + // 4: invalid product token -> USER_AGENT
		"total junk line here\n" + // 5: no event
		"Disallow: /caf\xC3\xA9\n" + // 6: strict rule -> escaped
		": nokey\n" + // 7: empty key, no event
		"Crawl-delay: 7\n" // 8: strict otherline -> UNKNOWN
	rec, err := g.Recover([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Strict != nil {
		t.Fatal("input contains junk; tier 2 expected")
	}
	want := []Event{
		{Line: 1, Kind: UserAgent, Value: "*"},
		{Line: 2, Kind: Disallow, Value: "/tmp"},
		{Line: 3, Kind: UserAgent, Value: "FooBot"},
		{Line: 4, Kind: UserAgent, Value: "Example Bot/1.0"},
		{Line: 6, Kind: Disallow, Value: "/caf%C3%A9"},
		{Line: 8, Kind: Unknown, Key: "Crawl-delay", Value: "7"},
	}
	if diffs := DiffEvents(rec.Events, want); len(diffs) != 0 {
		for _, d := range diffs {
			t.Errorf("golden (gluon | want): %s", d)
		}
	}
	// Line records: 8 lines, strict rules recorded where they matched.
	if len(rec.Lines) != 8 {
		t.Fatalf("got %d line records, want 8: %+v", len(rec.Lines), rec.Lines)
	}
	wantRules := []string{"startgroupline", "", "", "", "", "rule", "", "otherline"}
	for i, wr := range wantRules {
		if rec.Lines[i].Rule != wr {
			t.Errorf("line %d rule = %q (irregular=%v, reason=%q), want %q",
				i+1, rec.Lines[i].Rule, rec.Lines[i].Irregular, rec.Lines[i].Reason, wr)
		}
	}
}

// TestRecoverCrossGoogle is the phase-1 acceptance gate
// (docs/design/malformed-input.md): recovery events must equal google's for
// EVERY corpus file — the strict tier AND the malformed tier.
func TestRecoverCrossGoogle(t *testing.T) {
	g := mustGrammar(t)
	dump := findDumpBin(t)
	var files []string
	for _, pattern := range []string{"../testdata/*.txt", "../testdata/malformed/*.txt"} {
		fs, err := filepath.Glob(pattern)
		if err != nil || len(fs) == 0 {
			t.Fatalf("corpus glob %q failed: %v", pattern, err)
		}
		files = append(files, fs...)
	}
	for _, f := range files {
		f := f
		name, _ := filepath.Rel("../testdata", f)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			rec, err := g.Recover(src)
			if err != nil {
				t.Fatalf("Recover: %v", err)
			}
			googleEvents, err := GoogleEvents(dump, f)
			if err != nil {
				t.Fatalf("robots_dump: %v", err)
			}
			if diffs := DiffEvents(rec.Events, googleEvents); len(diffs) != 0 {
				t.Logf("recovery: %s", rec.RecoverSummary())
				for _, d := range diffs {
					t.Errorf("%s", d)
				}
			}
		})
	}
}
