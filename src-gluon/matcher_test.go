package robotsgluon

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findRobotsMain locates google's CLI (exit 0 allowed / 1 disallowed).
func findRobotsMain(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"../gen/bin/robots_main",
		"../bazel-bin/src-google/robots_main",
	} {
		if st, err := os.Stat(p); err == nil && st.Mode()&0o111 != 0 {
			return p
		}
	}
	t.Skip("robots_main not built (run ./build.sh); skipping matcher differential")
	return ""
}

func googleAllowed(t *testing.T, robotsMain, file, agent, url string) bool {
	t.Helper()
	cmd := exec.Command(robotsMain, file, agent, url)
	err := cmd.Run()
	if err == nil {
		return true
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false
	}
	t.Fatalf("robots_main %s %q %q: %v", file, agent, url, err)
	return false
}

func TestRobotsPatternMatches(t *testing.T) {
	cases := []struct {
		path, pattern string
		want          bool
	}{
		{"/", "/", true},
		{"/x", "/", true},
		{"/x", "/y", false},
		{"/fish.html", "/fish", true},
		{"/Fish.html", "/fish", false}, // case-sensitive
		{"/x/y.gif", "/*.gif", true},
		{"/x/y.gift", "/*.gif", true},   // '$'-less pattern is a prefix match
		{"/x/y.gift", "/*.gif$", false}, // anchored
		{"/x/y.gif", "/*.gif$", true},
		{"/", "", true}, // empty pattern matches everything
		{"/a/b", "/a/*/b", false},
		{"/a/x/b", "/a/*/b", true},
		{"/a/b", "/a$", false},
		{"/a", "/a$", true},
		{"/a$b", "/a$", false}, // '$' mid-path is literal in path, special in pattern end
	}
	for _, c := range cases {
		if got := robotsPatternMatches(c.path, c.pattern); got != c.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", c.path, c.pattern, got, c.want)
		}
	}
}

func TestGetPathParamsQuery(t *testing.T) {
	cases := map[string]string{
		"":                                         "/",
		"https://example.com":                      "/",
		"https://example.com/":                     "/",
		"https://example.com/a/b?q=1":              "/a/b?q=1",
		"https://example.com/a#frag":               "/a",
		"https://example.com#frag":                 "/",
		"//example.com/x":                          "/x",
		"example.com/x;p":                          "/x;p",
		"https://example.com?q":                    "/?q",
		"/already/a/path":                          "/already/a/path",
		"https://example.com/San%20Jos%C3%A9/page": "/San%20Jos%C3%A9/page",
	}
	for url, want := range cases {
		if got := getPathParamsQuery(url); got != want {
			t.Errorf("getPathParamsQuery(%q) = %q, want %q", url, got, want)
		}
	}
}

// TestMatcherCasesTSV replays the curated (file, agent, url, expected)
// tuples — expectations were recorded from robots_main itself
// (testdata/matcher-cases.tsv).
func TestMatcherCasesTSV(t *testing.T) {
	g := mustGrammar(t)
	f, err := os.Open("../testdata/matcher-cases.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			t.Fatalf("bad tsv line %q", line)
		}
		// The file column is relative to testdata/ (see the tsv header).
		file, agent, url, expect := parts[0], parts[1], parts[2], parts[3]
		src, err := os.ReadFile(filepath.Join("..", "testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		got, err := g.Allowed(src, agent, url)
		if err != nil {
			t.Fatalf("%s: %v", line, err)
		}
		want := expect == "allowed"
		if got != want {
			t.Errorf("%s %q %q: gluon says allowed=%v, want %s", file, agent, url, got, expect)
		}
		n++
	}
	if n == 0 {
		t.Fatal("no cases replayed")
	}
	t.Logf("replayed %d matcher cases", n)
}

// TestMatcherGridVsGoogle is the matcher's differential acceptance gate:
// for every corpus file (both tiers), a grid of agents × URLs derived from
// the file's own directives must produce identical decisions from our
// event-driven matcher and robots_main.
func TestMatcherGridVsGoogle(t *testing.T) {
	g := mustGrammar(t)
	robotsMain := findRobotsMain(t)
	var files []string
	for _, pattern := range []string{"../testdata/*.txt", "../testdata/malformed/*.txt"} {
		fs, _ := filepath.Glob(pattern)
		files = append(files, fs...)
	}
	if len(files) == 0 {
		t.Fatal("no corpus")
	}

	checked := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := g.Recover(src)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}

		// Agents: every product token in the file (so specific groups
		// engage) plus never-present and wildcard-ish agents.
		agents := map[string]bool{"FooBot": true, "absent-bot": true}
		// URLs: derived from every rule value (wildcards stripped for a
		// likely match, plus suffixed/mismatched variants) plus statics.
		urls := map[string]bool{
			"https://example.com/":            true,
			"https://example.com/x/unmatched": true,
		}
		for _, e := range rec.Events {
			switch e.Kind {
			case UserAgent:
				if tok := ExtractUserAgent(e.Value); tok != "" {
					agents[tok] = true
				}
			case Allow, Disallow:
				p := strings.ReplaceAll(strings.TrimSuffix(e.Value, "$"), "*", "x")
				if !strings.HasPrefix(p, "/") {
					p = "/" + p
				}
				urls["https://example.com"+p] = true
				urls["https://example.com"+p+"zz"] = true
			}
		}

		for agent := range agents {
			for url := range urls {
				ours, err := g.Allowed(src, agent, url)
				if err != nil {
					t.Fatalf("%s: %v", f, err)
				}
				google := googleAllowed(t, robotsMain, f, agent, url)
				if ours != google {
					t.Errorf("%s agent=%q url=%q: gluon allowed=%v, google allowed=%v",
						f, agent, url, ours, google)
				}
				checked++
			}
		}
	}
	t.Logf("checked %d (file, agent, url) triples against robots_main", checked)
}
