package robotsgluon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoogleLine(t *testing.T) {
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
	}
	for _, c := range cases {
		d := parseGoogleLine(c.line)
		if d.key != c.key || d.value != c.value || d.reason != c.reason {
			t.Errorf("%s: parseGoogleLine(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.name, c.line, d.key, d.value, d.reason, c.key, c.value, c.reason)
		}
	}
}

func TestClassifyKeyTypo(t *testing.T) {
	cases := []struct {
		key  string
		kind EventKind
		typo bool
	}{
		{"user-agent", UserAgent, false},
		{"User-Agent-Foo", UserAgent, false},
		{"useragent", UserAgent, true},
		{"user agent", UserAgent, true},
		{"allow", Allow, false},
		{"Allowed", Allow, false},
		{"disallow", Disallow, false},
		{"Dissallow", Disallow, true},
		{"disalow", Disallow, true},
		{"sitemap", Sitemap, false},
		{"site-map", Sitemap, true},
		{"crawl-delay", Unknown, false},
	}
	for _, c := range cases {
		kind, typo := classifyKeyTypo(c.key)
		if kind != c.kind || typo != c.typo {
			t.Errorf("classifyKeyTypo(%q) = (%s,%v), want (%s,%v)", c.key, kind, typo, c.kind, c.typo)
		}
	}
}

func TestGoogleLines(t *testing.T) {
	lines := googleLines([]byte("a\nb\r\nc\rd\n"))
	want := []struct {
		num   int32
		text  string
		final bool
	}{{1, "a", false}, {2, "b", false}, {3, "c", false}, {4, "d", false}, {5, "", true}}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d: %+v", len(lines), len(want), lines)
	}
	for i, w := range want {
		if lines[i].num != w.num || lines[i].text != w.text || lines[i].final != w.final {
			t.Errorf("line %d = {%d,%q,final=%v}, want {%d,%q,final=%v}",
				i, lines[i].num, lines[i].text, lines[i].final, w.num, w.text, w.final)
		}
	}
	// Unterminated tail is the final segment with content.
	lines = googleLines([]byte("x: y"))
	if len(lines) != 1 || lines[0].text != "x: y" || !lines[0].final {
		t.Fatalf("unterminated: %+v", lines)
	}
	// BOM is consumed before line 1.
	lines = googleLines([]byte("\xEF\xBB\xBFa\n"))
	if lines[0].text != "a" {
		t.Fatalf("BOM not consumed: %+v", lines)
	}
}

func TestGoogleLinesTooLong(t *testing.T) {
	long := strings.Repeat("a", maxLineLen+100)
	lines := googleLines([]byte("ok: 1\n" + long + "\nok: 2\n"))
	if lines[0].tooLong || lines[2].tooLong {
		t.Error("short lines flagged too-long")
	}
	if !lines[1].tooLong {
		t.Error("long line not flagged")
	}
	if len(lines[1].text) != maxLineLen-1 {
		t.Errorf("long line truncated to %d bytes, want %d", len(lines[1].text), maxLineLen-1)
	}
}

// TestRecoverStrictUsesTier1 pins that spec-valid input takes the strict
// path, produces the same events as Events(), and still carries metadata.
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
	// 3 content lines + google's phantom EOF line.
	if len(rec.Metadata) != 4 {
		t.Fatalf("metadata records = %d, want 4: %v", len(rec.Metadata), rec.Metadata)
	}
	if !rec.Metadata[3].IsEmpty || rec.Metadata[3].Line != 4 {
		t.Errorf("EOF metadata record wrong: %v", rec.Metadata[3])
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
	// Line records: 8 content lines (phantom EOF line excluded).
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
	// Metadata spot checks: 9 records (8 lines + EOF), typo flag on line 3.
	if len(rec.Metadata) != 9 {
		t.Fatalf("metadata records = %d, want 9", len(rec.Metadata))
	}
	if !rec.Metadata[1].IsMissingColonSeparator || !rec.Metadata[1].HasDirective {
		t.Errorf("line 2 metadata: %v", rec.Metadata[1])
	}
	if !rec.Metadata[2].IsAcceptableTypo {
		t.Errorf("line 3 should be flagged acceptable-typo: %v", rec.Metadata[2])
	}
}

// TestRecoverCrossGoogle is the acceptance gate for phases 1+2
// (docs/design/malformed-input.md): recovery events AND metadata must equal
// google's for EVERY corpus file — the strict tier AND the malformed tier.
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
			google, err := GoogleParse(dump, f)
			if err != nil {
				t.Fatalf("robots_dump: %v", err)
			}
			if diffs := DiffEvents(rec.Events, google.Events); len(diffs) != 0 {
				t.Logf("recovery: %s", rec.RecoverSummary())
				for _, d := range diffs {
					t.Errorf("event %s", d)
				}
			}
			if diffs := DiffMetadata(rec.Metadata, google.Metadata); len(diffs) != 0 {
				for _, d := range diffs {
					t.Errorf("%s", d)
				}
			}
		})
	}
}

// TestRecoverTooLongLine pins google's per-line length cap (phase 4): a
// line over kMaxLineLen bypasses tier 1 and both parsers agree on the
// truncated value and the too-long metadata flag.
func TestRecoverTooLongLine(t *testing.T) {
	g := mustGrammar(t)
	dump := findDumpBin(t)
	long := "/" + strings.Repeat("a", maxLineLen+500)
	src := []byte("User-agent: *\nDisallow: " + long + "\nAllow: /ok\n")
	f := filepath.Join(t.TempDir(), "long.txt")
	if err := os.WriteFile(f, src, 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := g.Recover(src)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Strict != nil {
		t.Fatal("too-long line must bypass tier 1 (google parses truncated content)")
	}
	google, err := GoogleParse(dump, f)
	if err != nil {
		t.Fatal(err)
	}
	if diffs := DiffEvents(rec.Events, google.Events); len(diffs) != 0 {
		for _, d := range diffs {
			t.Errorf("event %s", d)
		}
	}
	if diffs := DiffMetadata(rec.Metadata, google.Metadata); len(diffs) != 0 {
		for _, d := range diffs {
			t.Errorf("%s", d)
		}
	}
	if !rec.Metadata[1].IsLineTooLong {
		t.Error("line 2 not flagged too-long")
	}
}
