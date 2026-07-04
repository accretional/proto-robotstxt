package robotsgluon

import (
	"os"
	"path/filepath"
	"testing"
)

func mustGrammar(t *testing.T) *Grammar {
	t.Helper()
	g, err := Default()
	if err != nil {
		t.Fatalf("Default grammar: %v", err)
	}
	return g
}

func TestNormalize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty", "", ""},
		{"trailing nl kept", "a: b\n", "a: b\n"},
		{"nl appended", "a: b", "a: b\n"},
		{"cr kept", "a: b\r", "a: b\r"},
		{"bom stripped", "\xEF\xBB\xBFa: b\n", "a: b\n"},
		// google's BOM scan consumes matching prefix bytes even when the
		// full BOM never completes (robots.cc bom_pos post-increment).
		{"partial bom stripped", "\xEF\xBBa: b\n", "a: b\n"},
		{"partial bom one byte", "\xEFa: b\n", "a: b\n"},
		{"bom only", "\xEF\xBB\xBF", ""},
	}
	for _, c := range cases {
		if got := string(Normalize([]byte(c.in))); got != c.want {
			t.Errorf("%s: Normalize(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestParseValid(t *testing.T) {
	g := mustGrammar(t)
	cases := map[string]string{
		"empty":           "",
		"blank lines":     "\n\n\n",
		"minimal":         "User-agent: *\nDisallow: /\n",
		"no final nl":     "User-agent: *\nAllow: /",
		"crlf":            "User-agent: *\r\nDisallow: /private\r\n",
		"cr only":         "User-agent: *\rDisallow: /private\r",
		"case fold keys":  "USER-AGENT: FooBot\ndisallow: /\n",
		"multi ua":        "User-agent: a\nUser-agent: b\nDisallow: /x\n",
		"empty group":     "User-agent: lonely\n",
		"empty disallow":  "User-agent: *\nDisallow:\n",
		"comments":        "# hello\nUser-agent: * # trailing\nDisallow: / #c\n  # indented comment\n",
		"ws torture":      "  User-agent  :  \t*  \n\tDisallow\t:\t/a\t\n",
		"sitemap":         "User-agent: *\nAllow: /\n\nSitemap: https://accretional.com/sitemap.xml\n",
		"unknown keys":    "Crawl-delay: 10\nUser-agent: *\nHost: example.com\n",
		"stray rule":      "Disallow: /before-any-group\n",
		"utf8 path":       "User-agent: *\nDisallow: /caf\xC3\xA9\n",
		"wildcard other":  "User-agent: *\nDisallow: *.gif$\n", // otherline per RFC quirk
		"typo disallow":   "User-agent: *\nDissallow: /x\n",    // otherline, google typo
		"colon in value":  "Sitemap: https://x.example/a:b\n",
		"hash comment ws": "User-agent: *\nDisallow: /a#not-path\n",
	}
	for name, src := range cases {
		if _, err := g.Parse([]byte(src)); err != nil {
			t.Errorf("%s: Parse(%q) failed: %v", name, src, err)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	g := mustGrammar(t)
	cases := map[string]string{
		"missing colon":        "User-agent *\nDisallow: /\n",
		"ua with space":        "User-agent: Example Bot\n",
		"ua with digits":       "User-agent: bot123\n", // RFC identifier has no digits
		"junk line":            "this is not a directive\n",
		"empty key":            ": value\n",
		"useragent typo":       "useragent: foo\nDisallow: /\n", // rejected by other_key, no rule matches
		"invalid utf8 in path": "User-agent: *\nDisallow: /\xC3(\n",
		"bare nul":             "User-agent: *\nDisallow: /\x00\n",
	}
	for name, src := range cases {
		if _, err := g.Parse([]byte(src)); err == nil {
			t.Errorf("%s: Parse(%q) unexpectedly succeeded", name, src)
		}
	}
}

func TestEventsGolden(t *testing.T) {
	g := mustGrammar(t)
	src := "# preamble\n" + // line 1
		"User-Agent: FooBot # names bot\n" + // line 2: UA, comment stripped
		"Disallow: /priv%aate\xC3\xA9 \n" + // line 3: %aa uppercased, é escaped, trailing ws trimmed
		"allow: /ok\n" + // line 4: case-insensitive key
		"\n" + // line 5
		"Sitemap: https://example.com/s.xml\n" + // line 6: not escaped
		"Crawl-delay: 10\n" + // line 7: unknown
		"Dissallow: /typo\n" // line 8: google typo -> DISALLOW
	want := []Event{
		{Line: 2, Kind: UserAgent, Value: "FooBot"},
		{Line: 3, Kind: Disallow, Value: "/priv%AAte%C3%A9"},
		{Line: 4, Kind: Allow, Value: "/ok"},
		{Line: 6, Kind: Sitemap, Value: "https://example.com/s.xml"},
		{Line: 7, Kind: Unknown, Key: "Crawl-delay", Value: "10"},
		{Line: 8, Kind: Disallow, Value: "/typo"},
	}
	got, err := g.Events([]byte(src))
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if diffs := DiffEvents(got, want); len(diffs) != 0 {
		for _, d := range diffs {
			t.Errorf("golden mismatch (gluon | want): %s", d)
		}
	}
	// Key column of typed events isn't part of DiffEvents; assert the
	// Unknown key explicitly.
	if len(got) > 4 && got[4].Key != "Crawl-delay" {
		t.Errorf("unknown key = %q, want Crawl-delay", got[4].Key)
	}
}

// findDumpBin locates the robots_dump binary produced by build.sh (gen/bin)
// or a raw bazel build (bazel-bin).
func findDumpBin(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"../gen/bin/robots_dump",
		"../bazel-bin/tools/robots-dump/robots_dump",
	} {
		if st, err := os.Stat(p); err == nil && st.Mode()&0o111 != 0 {
			return p
		}
	}
	t.Skip("robots_dump binary not built (run ./build.sh); skipping cross-parser check")
	return ""
}

// TestCrossGoogle is the core conformance test: for every strict-tier corpus
// file, the gluon grammar parse compiled to events must equal google's
// deserialization of the same bytes.
func TestCrossGoogle(t *testing.T) {
	g := mustGrammar(t)
	dump := findDumpBin(t)
	files, err := filepath.Glob("../testdata/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("no testdata corpus found: %v", err)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			gluonEvents, err := g.Events(src)
			if err != nil {
				t.Fatalf("gluon parse failed (strict-tier corpus files must be RFC-valid, see testdata/README.md): %v", err)
			}
			googleEvents, err := GoogleEvents(dump, f)
			if err != nil {
				t.Fatalf("robots_dump failed: %v", err)
			}
			if diffs := DiffEvents(gluonEvents, googleEvents); len(diffs) != 0 {
				for _, d := range diffs {
					t.Errorf("%s", d)
				}
			}
		})
	}
}
