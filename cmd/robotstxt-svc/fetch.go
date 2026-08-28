package main

// fetch.go — fetching robots.txt, with the status-code semantics the spec
// defines for it.
//
// These semantics are the reason this service fetches its own input rather than
// being handed bytes: they are robots-specific, counter-intuitive (a 403 means
// *allow everything*), and getting them wrong silently changes what a crawler
// believes it may fetch. RFC 9309 §2.3.1, refined by Google's documented
// behavior — both summarized in this repo at docs/rfc/9309/README.md §8 and
// docs/google-dev-docs/crawling-docs-robots-txt-robots-txt-spec.md:
//
//	2xx                  -> success      parse the rules                (§2.3.1.1)
//	3xx                  -> follow up to 5 hops, then treat as 404      (§2.3.1.2)
//	4xx except 429       -> unavailable  allow all                      (§2.3.1.3)
//	429, 5xx             -> unreachable  disallow all                   (§2.3.1.4)
//	DNS/network/timeout  -> unreachable  (Google: "treated as a server error")
//
// The 30-day escape hatch in §2.3.1.4 (and Google's 12-hour/30-day schedule) is
// a caching policy over repeated fetches; this service is stateless and returns
// the initial verdict.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	pb "github.com/accretional/proto-robotstxt/proto/pb"
)

// DefaultUserAgent identifies this service to origins.
const DefaultUserAgent = "proto-robotstxt-svc/1.0 (+https://github.com/accretional/proto-robotstxt)"

// maxRobotsBytes is Google's 500 KiB robots.txt cap: "content which is after the
// maximum file size is ignored". Truncating rather than rejecting matches that.
const maxRobotsBytes = 500 << 10

// maxRedirects is the RFC's "at least five consecutive redirects" (§2.3.1.2).
const maxRedirects = 5

// errTooManyRedirects marks a chain that never resolved. The RFC lets a client
// treat that as unavailable, and Google explicitly treats it as a 404 — so it
// must be distinguishable from a transport failure, which is unreachable.
var errTooManyRedirects = errors.New("robots.txt redirect chain exceeded 5 hops")

type robotsFetch struct {
	Outcome    pb.FetchOutcome
	StatusCode int
	Body       []byte
	Err        error
}

type fetcher struct {
	client    *http.Client
	userAgent string
}

func newFetcher(userAgent string, timeout time.Duration) *fetcher {
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	return &fetcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return errTooManyRedirects
				}
				return nil
			},
		},
		userAgent: userAgent,
	}
}

// originOf normalizes whatever the caller called a "domain" into an origin.
// A bare host gets https, which is the only scheme worth defaulting to; an
// explicit scheme is honoured.
func originOf(domain string) (string, error) {
	d := strings.TrimSpace(domain)
	if d == "" {
		return "", errors.New("domain must not be empty")
	}
	if !strings.Contains(d, "://") {
		d = "https://" + d
	}
	u, err := url.Parse(d)
	if err != nil {
		return "", fmt.Errorf("parse domain: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in %q", domain)
	}
	return u.Scheme + "://" + u.Host, nil
}

// get fetches origin's robots.txt and classifies the result per §2.3.1.
func (f *fetcher) get(ctx context.Context, origin string) robotsFetch {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
	if err != nil {
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE, Err: err}
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/plain, */*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		// A chain that ran out of hops is "no valid robots.txt" (allow all);
		// anything else is a transport failure (disallow all).
		if errors.Is(err, errTooManyRedirects) {
			return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE, Err: err}
		}
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE, Err: err}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxRobotsBytes))
		if err != nil {
			return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE, StatusCode: resp.StatusCode, Err: err}
		}
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_SUCCESS, StatusCode: resp.StatusCode, Body: body}

	case resp.StatusCode == http.StatusTooManyRequests: // 429, grouped with 5xx
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE, StatusCode: resp.StatusCode}

	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE, StatusCode: resp.StatusCode}

	case resp.StatusCode >= 500:
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE, StatusCode: resp.StatusCode}

	default:
		// 1xx/3xx reaching here means the client stopped following without an
		// error; no rules were obtained, so treat it as no valid robots.txt.
		return robotsFetch{Outcome: pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE, StatusCode: resp.StatusCode}
	}
}
