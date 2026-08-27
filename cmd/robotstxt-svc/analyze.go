package main

// analyze.go — reducing an event stream to what a crawler needs up front:
// the sitemaps to walk, and the crawl delay to pace by.
//
// Both are read from the same parse-event stream the matcher consumes, so a
// document that is answerable for allow/disallow is answerable for these too —
// including a tier-2 (recovered) document.

import (
	"net/url"
	"strconv"
	"strings"

	robots "github.com/accretional/proto-robotstxt/src-gluon"
)

// sitemapsOf collects the Sitemap: values, resolved against origin and
// deduplicated with order preserved.
//
// Sitemap lines are document-global, not group-scoped: RFC 9309 §2.2.4 puts
// them outside the group grammar, so they are collected regardless of which
// user-agent group they appear in.
func sitemapsOf(events []robots.Event, origin string) []string {
	base, err := url.Parse(origin)
	if err != nil {
		base = nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, e := range events {
		if e.Kind != robots.Sitemap {
			continue
		}
		v := strings.TrimSpace(e.Value)
		if v == "" {
			continue
		}
		if base != nil {
			if u, err := url.Parse(v); err == nil {
				v = base.ResolveReference(u).String()
			}
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// crawlDelayOf returns the Crawl-delay that applies to agent, in seconds, or 0.
//
// Crawl-delay is not in RFC 9309 and google's parser does not act on it — it
// surfaces as an UNKNOWN event, which is exactly why the value has to be picked
// out here rather than read off a typed field. Two rules define "applies":
//
//   - Group membership follows the matcher's own grouping (a port of
//     robots.cc RobotsMatcher::HandleUserAgent): consecutive user-agent lines
//     merge into one group, and a user-agent line *after a rule* starts a new
//     one. Sharing that rule is what keeps the delay attributed to the same
//     group whose Allow/Disallow lines the matcher will apply.
//   - A delay in a group naming the agent beats one in the global ("*") group,
//     mirroring how a specific group outranks the global group for rules. Within
//     a precedence class the first well-formed value wins.
func crawlDelayOf(events []robots.Event, agent string) float64 {
	var (
		seenSpecific, seenGlobal, seenSeparator bool
		specific, global                        float64
		haveSpecific, haveGlobal                bool
	)
	token := robots.ExtractUserAgent(agent)

	for _, e := range events {
		switch e.Kind {
		case robots.UserAgent:
			// A new group begins only after a rule has been seen.
			if seenSeparator {
				seenSpecific, seenGlobal, seenSeparator = false, false, false
			}
			v := e.Value
			if len(v) >= 1 && v[0] == '*' && (len(v) == 1 || v[1] == ' ' || v[1] == '\t') {
				seenGlobal = true
				continue
			}
			if token != "" && asciiEqualFold(robots.ExtractUserAgent(v), token) {
				seenSpecific = true
			}

		case robots.Allow, robots.Disallow:
			if seenSpecific || seenGlobal {
				seenSeparator = true
			}

		case robots.Unknown:
			if !asciiEqualFold(e.Key, "crawl-delay") {
				continue
			}
			d, ok := parseDelay(e.Value)
			if !ok {
				continue
			}
			switch {
			case seenSpecific && !haveSpecific:
				specific, haveSpecific = d, true
			case seenGlobal && !seenSpecific && !haveGlobal:
				global, haveGlobal = d, true
			}
		}
	}

	if haveSpecific {
		return specific
	}
	if haveGlobal {
		return global
	}
	return 0
}

// parseDelay accepts the decimal forms Crawl-delay is written in. A negative or
// unparseable value is no value: there is no spec to appeal to, so the safe
// reading is that the site said nothing.
func parseDelay(v string) (float64, bool) {
	d, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || d < 0 {
		return 0, false
	}
	return d, true
}

// asciiEqualFold is ASCII-only case-insensitive equality, matching the fold the
// matcher uses (absl::EqualsIgnoreCase). strings.EqualFold would additionally
// apply Unicode simple folding, letting exotic agents match groups google never
// matches — a divergence src-gluon's port-fidelity review already called out.
func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
