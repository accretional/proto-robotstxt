package robotsgluon

// matcher.go — the allow/disallow decision logic, a byte-exact port of
// src-google/robots.cc RobotsMatcher. Like google's (which implements
// RobotsParseHandler), it consumes the parse-EVENT stream — the surface
// both parsers already agree on — so it works identically on strict-tier
// and recovered documents. Semantics ported:
//
//   - group tracking: consecutive user-agent lines merge; a user-agent
//     line after any rule (seen_separator) starts a new group; "*" (or
//     "* <junk>") is the global group
//   - agent matching: the line value's leading [a-zA-Z_-] token compared
//     case-insensitively against each caller agent
//   - longest-match precedence: pattern length is the priority; specific
//     group beats global; at equal priority allow wins (disallow only on
//     strictly greater priority); google's index.htm allow-normalization
//   - URL → path: GetPathParamsQuery (no percent-normalization of the URL —
//     the caller provides it escaped; patterns were escaped at parse time)

import "strings"

// Allowed reports whether userAgent may fetch url per robotsTxt, using the
// two-tier parse (total: any input yields a decision, like google). This
// mirrors robots_main's OneAgentAllowedByRobots.
func (g *Grammar) Allowed(robotsTxt []byte, userAgent, url string) (bool, error) {
	rec, err := g.Recover(robotsTxt)
	if err != nil {
		return false, err
	}
	return AllowedByEvents(rec.Events, []string{userAgent}, url), nil
}

// AllowedByEvents runs the matcher over an already-deserialized event
// stream for one or more caller user-agents (robots.cc AllowedByRobots).
func AllowedByEvents(events []Event, userAgents []string, url string) bool {
	m := robotsMatcher{
		agents:   userAgents,
		path:     getPathParamsQuery(url),
		allow:    newMatchHierarchy(),
		disallow: newMatchHierarchy(),
	}
	for _, e := range events {
		switch e.Kind {
		case UserAgent:
			m.handleUserAgent(e.Value)
		case Allow:
			m.handleAllow(e.Line, e.Value)
		case Disallow:
			m.handleDisallow(e.Line, e.Value)
		}
	}
	return !m.decideDisallow()
}

// match mirrors robots.cc RobotsMatcher::Match (priority + line).
type match struct {
	priority int // -1 = no match (kNoMatchPriority)
	line     int32
}

func (m *match) set(priority int, line int32) { m.priority, m.line = priority, line }

type matchHierarchy struct{ global, specific match }

type robotsMatcher struct {
	agents []string
	path   string

	allow, disallow matchHierarchy

	seenGlobalAgent   bool
	seenSpecificAgent bool
	everSeenSpecific  bool
	seenSeparator     bool
}

func newMatchHierarchy() matchHierarchy {
	return matchHierarchy{global: match{priority: -1}, specific: match{priority: -1}}
}

// handleUserAgent ports RobotsMatcher::HandleUserAgent, including the
// google-specific "'*' followed by space and more characters is still the
// global group" rule.
func (m *robotsMatcher) handleUserAgent(value string) {
	if m.seenSeparator {
		m.seenSpecificAgent, m.seenGlobalAgent, m.seenSeparator = false, false, false
	}
	if len(value) >= 1 && value[0] == '*' && (len(value) == 1 || isCSpace(value[1])) {
		m.seenGlobalAgent = true
		return
	}
	token := ExtractUserAgent(value)
	for _, agent := range m.agents {
		if strings.EqualFold(token, agent) {
			m.everSeenSpecific = true
			m.seenSpecificAgent = true
			break
		}
	}
}

func (m *robotsMatcher) seenAnyAgent() bool { return m.seenGlobalAgent || m.seenSpecificAgent }

// handleAllow ports RobotsMatcher::HandleAllow, including the recursion
// that normalizes a non-matching ".../index.htm*" pattern to ".../$".
func (m *robotsMatcher) handleAllow(line int32, value string) {
	if !m.seenAnyAgent() {
		return
	}
	m.seenSeparator = true
	priority := matchStrategy(m.path, value)
	if priority >= 0 {
		if m.seenSpecificAgent {
			if m.allow.specific.priority < priority {
				m.allow.specific.set(priority, line)
			}
		} else {
			if m.allow.global.priority < priority {
				m.allow.global.set(priority, line)
			}
		}
		return
	}
	// Google-specific optimization: 'index.htm' and 'index.html' are
	// normalized to '/'.
	slash := strings.LastIndexByte(value, '/')
	if slash >= 0 && strings.HasPrefix(value[slash:], "/index.htm") {
		m.handleAllow(line, value[:slash+1]+"$")
	}
}

// handleDisallow ports RobotsMatcher::HandleDisallow.
func (m *robotsMatcher) handleDisallow(line int32, value string) {
	if !m.seenAnyAgent() {
		return
	}
	m.seenSeparator = true
	priority := matchStrategy(m.path, value)
	if priority >= 0 {
		if m.seenSpecificAgent {
			if m.disallow.specific.priority < priority {
				m.disallow.specific.set(priority, line)
			}
		} else {
			if m.disallow.global.priority < priority {
				m.disallow.global.set(priority, line)
			}
		}
	}
}

// decideDisallow ports RobotsMatcher::disallow(): specific group first; a
// matching specific group without a winning disallow means allowed; global
// only consulted when no specific group was ever seen.
func (m *robotsMatcher) decideDisallow() bool {
	if m.allow.specific.priority > 0 || m.disallow.specific.priority > 0 {
		return m.disallow.specific.priority > m.allow.specific.priority
	}
	if m.everSeenSpecific {
		return false
	}
	if m.disallow.global.priority > 0 || m.allow.global.priority > 0 {
		return m.disallow.global.priority > m.allow.global.priority
	}
	return false
}

// matchStrategy ports LongestMatchRobotsMatchStrategy: the matched
// pattern's length is its priority, -1 when it doesn't match.
func matchStrategy(path, pattern string) int {
	if robotsPatternMatches(path, pattern) {
		return len(pattern)
	}
	return -1
}

// robotsPatternMatches ports RobotsMatchStrategy::Matches — anchored
// pattern matching where '*' matches any run and '$' is special only at
// the end of the pattern. The pos slice holds the sorted prefixes of path
// that can match the pattern prefix consumed so far.
func robotsPatternMatches(path, pattern string) bool {
	pathlen := len(path)
	pos := make([]int, pathlen+1)
	numpos := 1
	pos[0] = 0

	for i := 0; i < len(pattern); i++ {
		pat := pattern[i]
		if pat == '$' && i+1 == len(pattern) {
			return pos[numpos-1] == pathlen
		}
		if pat == '*' {
			numpos = pathlen - pos[0] + 1
			for j := 1; j < numpos; j++ {
				pos[j] = pos[j-1] + 1
			}
		} else {
			newnumpos := 0
			for j := 0; j < numpos; j++ {
				if pos[j] < pathlen && path[pos[j]] == pat {
					pos[newnumpos] = pos[j] + 1
					newnumpos++
				}
			}
			numpos = newnumpos
			if numpos == 0 {
				return false
			}
		}
	}
	return true
}

// ExtractUserAgent ports RobotsMatcher::ExtractUserAgent: the longest
// prefix of [a-zA-Z_-] characters (the product token google matches on).
func ExtractUserAgent(userAgent string) string {
	i := 0
	for i < len(userAgent) {
		c := userAgent[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_' {
			i++
			continue
		}
		break
	}
	return userAgent[:i]
}

// isCSpace mirrors C isspace() for the "'*' followed by space" check.
func isCSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// getPathParamsQuery ports robots.cc GetPathParamsQuery: extract path (with
// params) and query from a URL, dropping scheme, authority and fragment.
// The result always starts with "/"; "/" when the url has no usable path.
func getPathParamsQuery(url string) string {
	searchStart := 0
	if len(url) >= 2 && url[0] == '/' && url[1] == '/' {
		searchStart = 2
	}
	earlyPath := indexAnyFrom(url, "/?;", searchStart)
	protocolEnd := indexFrom(url, "://", searchStart)
	if earlyPath >= 0 && (protocolEnd < 0 || earlyPath < protocolEnd) {
		// A path/param/query before "://" means "://" is not a protocol.
		protocolEnd = -1
	}
	if protocolEnd < 0 {
		protocolEnd = searchStart
	} else {
		protocolEnd += 3
	}

	pathStart := indexAnyFrom(url, "/?;", protocolEnd)
	if pathStart >= 0 {
		hashPos := indexFrom(url, "#", searchStart)
		if hashPos >= 0 && hashPos < pathStart {
			return "/"
		}
		pathEnd := len(url)
		if hashPos >= 0 {
			pathEnd = hashPos
		}
		if url[pathStart] != '/' {
			// Prepend a slash if the result would start e.g. with '?'.
			return "/" + url[pathStart:pathEnd]
		}
		return url[pathStart:pathEnd]
	}
	return "/"
}

func indexAnyFrom(s, chars string, from int) int {
	if from >= len(s) {
		return -1
	}
	if i := strings.IndexAny(s[from:], chars); i >= 0 {
		return from + i
	}
	return -1
}

func indexFrom(s, sub string, from int) int {
	if from >= len(s) {
		return -1
	}
	if i := strings.Index(s[from:], sub); i >= 0 {
		return from + i
	}
	return -1
}
