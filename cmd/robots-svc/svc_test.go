package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	robots "github.com/accretional/proto-robotstxt/src-gluon"
)

func grammar(t *testing.T) *robots.Grammar {
	t.Helper()
	g, err := robots.Default()
	if err != nil {
		t.Fatalf("grammar: %v", err)
	}
	return g
}

func eventsOf(t *testing.T, g *robots.Grammar, src string) []robots.Event {
	t.Helper()
	rec, err := g.Recover([]byte(src))
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	return rec.Events
}

// --- §2.3.1 status-code semantics -------------------------------------------

// The counter-intuitive half of the spec: 4xx means allow everything, 5xx means
// disallow everything. Getting these backwards silently changes what a crawler
// believes it may fetch, so each is pinned.
func TestFetch_StatusSemantics(t *testing.T) {
	cases := []struct {
		status  int
		outcome FetchOutcome
	}{
		{200, OutcomeSuccess},
		{204, OutcomeSuccess},
		{401, OutcomeUnavailable}, // google explicitly warns 401/403 mean ALLOW
		{403, OutcomeUnavailable},
		{404, OutcomeUnavailable},
		{410, OutcomeUnavailable},
		{429, OutcomeUnreachable}, // the 4xx exception, grouped with 5xx
		{500, OutcomeUnreachable},
		{503, OutcomeUnreachable},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			got := newFetcher("test", 5*time.Second).get(context.Background(), srv.URL)
			if got.Outcome != tc.outcome {
				t.Errorf("HTTP %d -> %s, want %s", tc.status, got.Outcome, tc.outcome)
			}
		})
	}
}

// A redirect chain that never resolves is "no valid robots.txt" (allow all),
// which must not be confused with a transport failure (disallow all).
func TestFetch_RedirectChainExhaustsToUnavailable(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/next", http.StatusFound)
	}))
	defer srv.Close()

	got := newFetcher("test", 5*time.Second).get(context.Background(), srv.URL)
	if got.Outcome != OutcomeUnavailable {
		t.Errorf("endless redirect -> %s, want %s", got.Outcome, OutcomeUnavailable)
	}
}

// A redirect that resolves within the budget is followed.
func TestFetch_RedirectFollowedWithinBudget(t *testing.T) {
	var srv *httptest.Server
	hops := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hops < 3 {
			hops++
			http.Redirect(w, r, srv.URL+"/hop", http.StatusFound)
			return
		}
		w.Write([]byte("User-agent: *\nDisallow: /x\n"))
	}))
	defer srv.Close()

	got := newFetcher("test", 5*time.Second).get(context.Background(), srv.URL)
	if got.Outcome != OutcomeSuccess || !strings.Contains(string(got.Body), "Disallow") {
		t.Errorf("3-hop redirect -> %s (%q), want success with the body", got.Outcome, got.Body)
	}
}

// DNS/network failure is a server error, per google: disallow all.
func TestFetch_NetworkFailureIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	got := newFetcher("test", 2*time.Second).get(context.Background(), url)
	if got.Outcome != OutcomeUnreachable {
		t.Errorf("dead host -> %s, want %s", got.Outcome, OutcomeUnreachable)
	}
}

// Google ignores content past 500 KiB rather than rejecting the file.
func TestFetch_TruncatesAtGoogleSizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\n"))
		w.Write([]byte(strings.Repeat("# padding padding padding\n", 40000))) // > 500 KiB
	}))
	defer srv.Close()

	got := newFetcher("test", 10*time.Second).get(context.Background(), srv.URL)
	if got.Outcome != OutcomeSuccess {
		t.Fatalf("outcome = %s, want success", got.Outcome)
	}
	if len(got.Body) != maxRobotsBytes {
		t.Errorf("body = %d bytes, want it truncated to %d", len(got.Body), maxRobotsBytes)
	}
}

func TestOriginOf(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.com", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"http://example.com", "http://example.com"},
		{"https://example.com/some/page?q=1", "https://example.com"},
		{"example.com:8443", "https://example.com:8443"},
		{"  example.com  ", "https://example.com"},
	}
	for _, tc := range cases {
		got, err := originOf(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("originOf(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "   ", "ftp://example.com", "https://"} {
		if _, err := originOf(bad); err == nil {
			t.Errorf("originOf(%q) should error", bad)
		}
	}
}

// --- sitemaps ---------------------------------------------------------------

func TestSitemapsOf(t *testing.T) {
	g := grammar(t)
	src := "Sitemap: https://example.com/a.xml\n" +
		"User-agent: *\n" +
		"Disallow: /private\n" +
		"Sitemap: /relative.xml\n" +
		"Sitemap: https://example.com/a.xml\n" // duplicate

	got := sitemapsOf(eventsOf(t, g, src), "https://example.com")
	want := []string{"https://example.com/a.xml", "https://example.com/relative.xml"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("sitemaps = %v, want %v (resolved, deduped, order preserved)", got, want)
	}
}

// --- crawl delay ------------------------------------------------------------

func TestCrawlDelayOf(t *testing.T) {
	g := grammar(t)
	cases := []struct {
		name  string
		src   string
		agent string
		want  float64
	}{
		{
			name:  "global group applies to any agent",
			src:   "User-agent: *\nCrawl-delay: 5\nDisallow: /x\n",
			agent: "MyBot",
			want:  5,
		},
		{
			name:  "specific group beats global",
			src:   "User-agent: *\nCrawl-delay: 5\nDisallow: /x\n\nUser-agent: MyBot\nCrawl-delay: 1.5\nDisallow: /y\n",
			agent: "MyBot",
			want:  1.5,
		},
		{
			name:  "specific group wins even when it appears first",
			src:   "User-agent: MyBot\nCrawl-delay: 2\nDisallow: /y\n\nUser-agent: *\nCrawl-delay: 9\nDisallow: /x\n",
			agent: "MyBot",
			want:  2,
		},
		{
			name:  "agent match is case-insensitive",
			src:   "User-agent: mybot\nCrawl-delay: 3\nDisallow: /\n",
			agent: "MyBot",
			want:  3,
		},
		{
			name:  "merged consecutive agents share the group's delay",
			src:   "User-agent: A\nUser-agent: MyBot\nCrawl-delay: 7\nDisallow: /\n",
			agent: "MyBot",
			want:  7,
		},
		{
			name:  "a user-agent after a rule starts a new group",
			src:   "User-agent: MyBot\nDisallow: /\nUser-agent: Other\nCrawl-delay: 8\nDisallow: /\n",
			agent: "MyBot",
			want:  0,
		},
		{
			name:  "no crawl-delay at all",
			src:   "User-agent: *\nDisallow: /x\n",
			agent: "MyBot",
			want:  0,
		},
		{
			name:  "unparseable and negative values are no value",
			src:   "User-agent: *\nCrawl-delay: soon\nCrawl-delay: -4\nCrawl-delay: 2\nDisallow: /\n",
			agent: "MyBot",
			want:  2,
		},
		{
			name:  "a delay before any user-agent belongs to no group",
			src:   "Crawl-delay: 9\nUser-agent: *\nDisallow: /\n",
			agent: "MyBot",
			want:  0,
		},
		{
			name:  "another agent's group does not apply",
			src:   "User-agent: Other\nCrawl-delay: 6\nDisallow: /\n",
			agent: "MyBot",
			want:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := crawlDelayOf(eventsOf(t, g, tc.src), tc.agent); got != tc.want {
				t.Errorf("crawlDelayOf = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- handlers ---------------------------------------------------------------

func newServer(t *testing.T) *server {
	t.Helper()
	return &server{grammar: grammar(t), fetch: newFetcher("test", 5*time.Second)}
}

func TestFilterHandler(t *testing.T) {
	s := newServer(t)
	body := `{"robots_txt":"User-agent: *\nDisallow: /private\nAllow: /private/ok\n",
	          "agent":"MyBot",
	          "urls":["https://e.com/public","https://e.com/private/x","https://e.com/private/ok"]}`

	rr := httptest.NewRecorder()
	s.filter(rr, httptest.NewRequest("POST", "/v1/robots:filter", strings.NewReader(body)))
	if rr.Code != 200 {
		t.Fatalf("code = %d: %s", rr.Code, rr.Body)
	}
	got := rr.Body.String()
	for _, want := range []string{`"https://e.com/public"`, `"https://e.com/private/ok"`} {
		if !strings.Contains(got, want) {
			t.Errorf("response %s missing allowed %s", got, want)
		}
	}
	if !strings.Contains(got, `"disallowed":["https://e.com/private/x"]`) {
		t.Errorf("response %s should disallow /private/x only", got)
	}
}

// The §2.3.1 verdicts short-circuit the matcher, so a caller can act on an
// unavailable or unreachable robots.txt without inventing a rules document.
func TestFilterHandler_ShortCircuits(t *testing.T) {
	s := newServer(t)
	urls := `["https://e.com/a","https://e.com/b"]`

	rr := httptest.NewRecorder()
	s.filter(rr, httptest.NewRequest("POST", "/", strings.NewReader(`{"allow_all":true,"urls":`+urls+`}`)))
	if !strings.Contains(rr.Body.String(), `"allowed":["https://e.com/a","https://e.com/b"]`) {
		t.Errorf("allow_all: %s", rr.Body)
	}

	rr = httptest.NewRecorder()
	s.filter(rr, httptest.NewRequest("POST", "/", strings.NewReader(`{"disallow_all":true,"urls":`+urls+`}`)))
	if !strings.Contains(rr.Body.String(), `"disallowed":["https://e.com/a","https://e.com/b"]`) {
		t.Errorf("disallow_all: %s", rr.Body)
	}

	rr = httptest.NewRecorder()
	s.filter(rr, httptest.NewRequest("POST", "/", strings.NewReader(`{"allow_all":true,"disallow_all":true,"urls":`+urls+`}`)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("contradictory verdicts should be rejected; got %d", rr.Code)
	}
}

// End-to-end over a live origin: a document that fails the strict RFC grammar
// still answers, and says it was recovered.
func TestParseHandler_RecoveredTierStillAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "archive.org_bot" is not a legal RFC 9309 product-token ('.' and digits
		// are outside the grammar), so tier 1 rejects the document.
		w.Write([]byte("User-agent: archive.org_bot\nDisallow: /\n\n" +
			"User-agent: *\nCrawl-delay: 2\nDisallow: /private\n" +
			"Sitemap: /sitemap.xml\n"))
	}))
	defer srv.Close()

	s := newServer(t)
	rr := httptest.NewRecorder()
	s.parse(rr, httptest.NewRequest("POST", "/", strings.NewReader(
		`{"domain":"`+srv.URL+`","agent":"MyBot","include_text":true}`)))

	if rr.Code != 200 {
		t.Fatalf("code = %d: %s", rr.Code, rr.Body)
	}
	got := rr.Body.String()
	for _, want := range []string{
		`"outcome":"success"`,
		`"tier":"recovered"`,
		`"crawl_delay_seconds":2`,
		srv.URL + `/sitemap.xml`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("response missing %s:\n%s", want, got)
		}
	}
}

// A 403 yields allow-all with no rules, not an error.
func TestParseHandler_UnavailableAllowsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := newServer(t)
	rr := httptest.NewRecorder()
	s.parse(rr, httptest.NewRequest("POST", "/", strings.NewReader(`{"domain":"`+srv.URL+`"}`)))

	got := rr.Body.String()
	if !strings.Contains(got, `"outcome":"unavailable"`) || !strings.Contains(got, `"allow_all":true`) {
		t.Errorf("403 should be unavailable/allow_all: %s", got)
	}
}
