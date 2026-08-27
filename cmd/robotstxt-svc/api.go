package main

// api.go — the HTTP/JSON surface of robotstxt-svc.
//
// The wire types are plain Go structs, not protos. proto/rep.proto and
// proto/recover.proto are *generated from the grammar* (CLAUDE.md rule 6:
// regenerate, don't hand-edit), so a hand-written service message in proto/
// would blur what that directory means and risk being clobbered by the next
// `gluon genproto`. The robots.txt *domain* stays proto; only this transport
// envelope is Go.

// FetchOutcome is how a robots.txt fetch resolved, in RFC 9309 §2.3.1 terms.
type FetchOutcome string

const (
	// OutcomeSuccess: rules were fetched and parsed (§2.3.1.1).
	OutcomeSuccess FetchOutcome = "success"
	// OutcomeUnavailable: no valid robots.txt exists — 4xx other than 429, or a
	// redirect chain that never resolved. The crawler MAY access any resource
	// (§2.3.1.3): allow all.
	OutcomeUnavailable FetchOutcome = "unavailable"
	// OutcomeUnreachable: robots.txt is undefined — 429, 5xx, DNS or network
	// failure. The crawler MUST assume complete disallow (§2.3.1.4).
	OutcomeUnreachable FetchOutcome = "unreachable"
)

// ParseRequest asks for a domain's robots.txt: fetched, parsed, and reduced to
// what a crawler needs before it starts.
type ParseRequest struct {
	// Domain is a bare host ("example.com"), an origin, or any URL — the origin
	// is derived from it. A missing scheme defaults to https.
	Domain string `json:"domain"`
	// Agent is the product token whose group applies (default "*").
	Agent string `json:"agent,omitempty"`
	// IncludeText returns the raw robots.txt in the response, so a caller can
	// pass it straight to :filter without a second fetch.
	IncludeText bool `json:"include_text,omitempty"`
}

// ParseResponse is what one robots.txt says to one agent.
type ParseResponse struct {
	Origin    string       `json:"origin"`
	RobotsURL string       `json:"robots_url"`
	Outcome   FetchOutcome `json:"outcome"`

	StatusCode int    `json:"status_code,omitempty"`
	FetchError string `json:"fetch_error,omitempty"`

	// AllowAll / DisallowAll carry the §2.3.1 verdict when there are no rules to
	// apply. Exactly one is true unless Outcome is success.
	AllowAll    bool `json:"allow_all"`
	DisallowAll bool `json:"disallow_all"`

	// Sitemaps are the Sitemap: lines, resolved against the origin. They are
	// document-global, not group-scoped (RFC 9309 §2.2.4).
	Sitemaps []string `json:"sitemaps"`

	// CrawlDelaySeconds is the Crawl-delay of the group matching Agent, if any.
	// It is not part of RFC 9309 and google's parser ignores it; see analyze.go.
	CrawlDelaySeconds float64 `json:"crawl_delay_seconds,omitempty"`

	// Tier is which parse tier produced the events: "strict" (the document
	// satisfies the RFC 9309 grammar) or "recovered" (per-line recovery).
	Tier string `json:"tier"`
	// IrregularLines counts lines no grammar rule matched, recovered by the
	// robots.cc fallback. A useful health signal about a site's robots.txt.
	IrregularLines int `json:"irregular_lines"`

	RobotsTxt string `json:"robots_txt,omitempty"`
}

// FilterRequest applies a robots.txt to a list of URLs — the join step that
// turns "URLs a sitemap lists" into "URLs this agent may fetch".
type FilterRequest struct {
	RobotsTxt string   `json:"robots_txt,omitempty"`
	Agent     string   `json:"agent,omitempty"`
	URLs      []string `json:"urls"`
	// AllowAll / DisallowAll short-circuit the matcher when the caller already
	// has a §2.3.1 verdict from :parse and no rules to apply.
	AllowAll    bool `json:"allow_all,omitempty"`
	DisallowAll bool `json:"disallow_all,omitempty"`
}

// FilterResponse partitions the requested URLs.
type FilterResponse struct {
	Allowed    []string `json:"allowed"`
	Disallowed []string `json:"disallowed"`
}
