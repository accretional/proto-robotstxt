package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/accretional/proto-robotstxt/proto/pb"
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
		outcome pb.FetchOutcome
	}{
		{200, pb.FetchOutcome_FETCH_OUTCOME_SUCCESS},
		{204, pb.FetchOutcome_FETCH_OUTCOME_SUCCESS},
		{401, pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE}, // google explicitly warns 401/403 mean ALLOW
		{403, pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE},
		{404, pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE},
		{410, pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE},
		{429, pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE}, // the 4xx exception, grouped with 5xx
		{500, pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE},
		{503, pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE},
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
	if got.Outcome != pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE {
		t.Errorf("endless redirect -> %s, want %s", got.Outcome, pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE)
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
	if got.Outcome != pb.FetchOutcome_FETCH_OUTCOME_SUCCESS || !strings.Contains(string(got.Body), "Disallow") {
		t.Errorf("3-hop redirect -> %s (%q), want success with the body", got.Outcome, got.Body)
	}
}

// DNS/network failure is a server error, per google: disallow all.
func TestFetch_NetworkFailureIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	got := newFetcher("test", 2*time.Second).get(context.Background(), url)
	if got.Outcome != pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE {
		t.Errorf("dead host -> %s, want %s", got.Outcome, pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE)
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
	if got.Outcome != pb.FetchOutcome_FETCH_OUTCOME_SUCCESS {
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

func TestFilter(t *testing.T) {
	s := newServer(t)
	resp, err := s.Filter(context.Background(), &pb.FilterRequest{
		RobotsTxt: "User-agent: *\nDisallow: /private\nAllow: /private/ok\n",
		Agent:     "MyBot",
		Urls: []string{
			"https://e.com/public",
			"https://e.com/private/x",
			"https://e.com/private/ok",
		},
	})
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(resp.Allowed) != 2 {
		t.Errorf("allowed = %v, want /public and /private/ok", resp.Allowed)
	}
	if len(resp.Disallowed) != 1 || resp.Disallowed[0] != "https://e.com/private/x" {
		t.Errorf("disallowed = %v, want just /private/x", resp.Disallowed)
	}
}

// The §2.3.1 verdicts short-circuit the matcher, so a caller can act on an
// unavailable or unreachable robots.txt without inventing a rules document.
func TestFilter_ShortCircuits(t *testing.T) {
	s := newServer(t)
	urls := []string{"https://e.com/a", "https://e.com/b"}

	resp, err := s.Filter(context.Background(), &pb.FilterRequest{AllowAll: true, Urls: urls})
	if err != nil || len(resp.Allowed) != 2 {
		t.Errorf("allow_all: %v %v", resp, err)
	}

	resp, err = s.Filter(context.Background(), &pb.FilterRequest{DisallowAll: true, Urls: urls})
	if err != nil || len(resp.Disallowed) != 2 {
		t.Errorf("disallow_all: %v %v", resp, err)
	}

	_, err = s.Filter(context.Background(), &pb.FilterRequest{AllowAll: true, DisallowAll: true, Urls: urls})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("contradictory verdicts should be InvalidArgument; got %v", err)
	}
}

// End-to-end over a live origin: a document that fails the strict RFC grammar
// still answers, and says it was recovered.
func TestParse_RecoveredTierStillAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// "archive.org_bot" is not a legal RFC 9309 product-token ('.' and digits
		// are outside the grammar), so tier 1 rejects the document.
		w.Write([]byte("User-agent: archive.org_bot\nDisallow: /\n\n" +
			"User-agent: *\nCrawl-delay: 2\nDisallow: /private\n" +
			"Sitemap: /sitemap.xml\n"))
	}))
	defer srv.Close()

	s := newServer(t)
	resp, err := s.Parse(context.Background(), &pb.ParseRequest{
		Domain: srv.URL, Agent: "MyBot", IncludeText: true,
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Outcome != pb.FetchOutcome_FETCH_OUTCOME_SUCCESS || resp.Tier != "recovered" {
		t.Errorf("outcome=%s tier=%s, want success/recovered", resp.Outcome, resp.Tier)
	}
	if resp.CrawlDelaySeconds != 2 {
		t.Errorf("crawl delay = %v, want 2", resp.CrawlDelaySeconds)
	}
	if len(resp.Sitemaps) != 1 || resp.Sitemaps[0] != srv.URL+"/sitemap.xml" {
		t.Errorf("sitemaps = %v, want the resolved /sitemap.xml", resp.Sitemaps)
	}
	if resp.RobotsTxt == "" {
		t.Error("include_text set but robots_txt empty")
	}
}

// A 403 yields allow-all with no rules, not an error.
func TestParse_UnavailableAllowsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	s := newServer(t)
	resp, err := s.Parse(context.Background(), &pb.ParseRequest{Domain: srv.URL})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Outcome != pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE || !resp.AllowAll {
		t.Errorf("403 should be unavailable/allow_all; got %s allow_all=%v", resp.Outcome, resp.AllowAll)
	}
}

// A bad domain is an InvalidArgument, not an internal error.
func TestParse_BadDomainIsInvalidArgument(t *testing.T) {
	s := newServer(t)
	if _, err := s.Parse(context.Background(), &pb.ParseRequest{Domain: "ftp://example.com"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("got %v, want InvalidArgument", err)
	}
}

// The real gRPC surface: served over a socket, reflection registered, health
// reporting SERVING. This is what grpcurl and Cloud Run actually talk to.
func TestGRPCSurface(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterRobotsServiceServer(srv, newServer(t))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := pb.NewRobotsServiceClient(conn).Filter(ctx, &pb.FilterRequest{
		RobotsTxt: "User-agent: *\nDisallow: /private\n",
		Urls:      []string{"https://e.com/ok", "https://e.com/private/x"},
	})
	if err != nil {
		t.Fatalf("Filter over the wire: %v", err)
	}
	if len(got.Allowed) != 1 || len(got.Disallowed) != 1 {
		t.Errorf("allowed=%v disallowed=%v, want one each", got.Allowed, got.Disallowed)
	}

	hc, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil || hc.Status != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("health = %v, %v; want SERVING", hc.GetStatus(), err)
	}
}
