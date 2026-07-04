package robotsgluon

// matchers.go — token matchers for the lexical atoms of grammar/rep.ebnf.
//
// Each matcher implements EXACTLY the ABNF quoted on its grammar rule (RFC
// 9309 §2.2); rep.ebnf declares those rules with empty bodies and the parser
// resolves them here via metaparser.ParseCSTWithOptions. A matcher receives
// (src, pos) and returns the matched text plus the new position, or ("", -1)
// for no match. Matchers see the raw normalized source — no whitespace is
// skipped around them (the grammar is parsed whitespace-significant).

import (
	"strings"

	"github.com/accretional/gluon/v2/metaparser"
)

// Matchers returns the token-matcher table for grammar/rep.ebnf. The map is
// freshly allocated so callers may extend it without aliasing.
func Matchers() map[string]metaparser.TokenMatchFunc {
	return map[string]metaparser.TokenMatchFunc{
		"ws":              matchWS,
		"nl":              matchNL,
		"useragent_key":   matchCaseInsensitiveLiteral("user-agent"),
		"allow_key":       matchCaseInsensitiveLiteral("allow"),
		"disallow_key":    matchCaseInsensitiveLiteral("disallow"),
		"sitemap_key":     matchCaseInsensitiveLiteral("sitemap"),
		"utf8_char_noctl": matchUTF8CharNoctl,
		"comment_char":    matchCommentChar,
		"other_key":       matchOtherKey,
		"any_value":       matchAnyValue,
	}
}

// isWS reports ABNF WS: %x20 / %x09.
func isWS(b byte) bool { return b == ' ' || b == '\t' }

// matchWS implements 1*(%x20 / %x09). The grammar wraps every use in [ ]
// where the RFC says *WS, so the matcher itself must not match empty (a
// zero-length token would defeat the parser's infinite-loop guard).
func matchWS(src string, pos int) (string, int) {
	end := pos
	for end < len(src) && isWS(src[end]) {
		end++
	}
	if end == pos {
		return "", -1
	}
	return src[pos:end], end
}

// matchNL implements NL = %x0D / %x0A / %x0D.0A — longest match first, so
// CRLF is one token (google's parser likewise treats \r\n as one line break).
func matchNL(src string, pos int) (string, int) {
	if pos < len(src) && src[pos] == '\r' {
		if pos+1 < len(src) && src[pos+1] == '\n' {
			return src[pos : pos+2], pos + 2
		}
		return src[pos : pos+1], pos + 1
	}
	if pos < len(src) && src[pos] == '\n' {
		return src[pos : pos+1], pos + 1
	}
	return "", -1
}

// matchCaseInsensitiveLiteral matches an ABNF quoted string, which per RFC
// 5234 §2.3 is case-insensitive. The matched text keeps the input's casing.
func matchCaseInsensitiveLiteral(lit string) metaparser.TokenMatchFunc {
	return func(src string, pos int) (string, int) {
		end := pos + len(lit)
		if end > len(src) || !strings.EqualFold(src[pos:end], lit) {
			return "", -1
		}
		return src[pos:end], end
	}
}

// utf8CharNoctlLen returns the byte length of one UTF8-char-noctl at src[pos]
// (0 if none). Implements RFC 9309's class byte-for-byte:
//
//	UTF8-1-noctl = %x21 / %x22 / %x24-7F   (excludes controls, SP, "#")
//	UTF8-2       = %xC2-DF UTF8-tail
//	UTF8-3       = %xE0 %xA0-BF UTF8-tail / %xE1-EC 2UTF8-tail /
//	               %xED %x80-9F UTF8-tail / %xEE-EF 2UTF8-tail
//	UTF8-4       = %xF0 %x90-BF 2UTF8-tail / %xF1-F3 3UTF8-tail /
//	               %xF4 %x80-8F 2UTF8-tail
//	UTF8-tail    = %x80-BF
//
// Note DEL (%x7F) is allowed by the RFC's class and therefore here too.
func utf8CharNoctlLen(src string, pos int) int {
	if pos >= len(src) {
		return 0
	}
	b0 := src[pos]
	tail := func(off int) bool {
		return pos+off < len(src) && src[pos+off] >= 0x80 && src[pos+off] <= 0xBF
	}
	switch {
	case b0 == 0x21 || b0 == 0x22 || (b0 >= 0x24 && b0 <= 0x7F):
		return 1
	case b0 >= 0xC2 && b0 <= 0xDF:
		if tail(1) {
			return 2
		}
	case b0 == 0xE0:
		if pos+1 < len(src) && src[pos+1] >= 0xA0 && src[pos+1] <= 0xBF && tail(2) {
			return 3
		}
	case (b0 >= 0xE1 && b0 <= 0xEC) || b0 == 0xEE || b0 == 0xEF:
		if tail(1) && tail(2) {
			return 3
		}
	case b0 == 0xED:
		if pos+1 < len(src) && src[pos+1] >= 0x80 && src[pos+1] <= 0x9F && tail(2) {
			return 3
		}
	case b0 == 0xF0:
		if pos+1 < len(src) && src[pos+1] >= 0x90 && src[pos+1] <= 0xBF && tail(2) && tail(3) {
			return 4
		}
	case b0 >= 0xF1 && b0 <= 0xF3:
		if tail(1) && tail(2) && tail(3) {
			return 4
		}
	case b0 == 0xF4:
		if pos+1 < len(src) && src[pos+1] >= 0x80 && src[pos+1] <= 0x8F && tail(2) && tail(3) {
			return 4
		}
	}
	return 0
}

// matchUTF8CharNoctl matches exactly one UTF8-char-noctl.
func matchUTF8CharNoctl(src string, pos int) (string, int) {
	n := utf8CharNoctlLen(src, pos)
	if n == 0 {
		return "", -1
	}
	return src[pos : pos+n], pos + n
}

// matchCommentChar matches one character of a comment body:
// UTF8-char-noctl / WS / "#".
func matchCommentChar(src string, pos int) (string, int) {
	if pos < len(src) && (isWS(src[pos]) || src[pos] == '#') {
		return src[pos : pos+1], pos + 1
	}
	return matchUTF8CharNoctl(src, pos)
}

// keyValueRun matches a run of (UTF8-char-noctl / WS) starting at pos with
// the bytes in `excluded` additionally rejected, then right-trims WS. It
// returns the trimmed text and the position just past it, or ("", -1) when
// the trimmed run is empty. Shared shape of other_key / any_value.
func keyValueRun(src string, pos int, excluded string) (string, int) {
	end := pos
	for end < len(src) {
		if isWS(src[end]) {
			end++
			continue
		}
		if strings.IndexByte(excluded, src[end]) >= 0 {
			break
		}
		n := utf8CharNoctlLen(src, end)
		if n == 0 {
			break
		}
		end += n
	}
	for end > pos && isWS(src[end-1]) {
		end--
	}
	if end == pos {
		return "", -1
	}
	return src[pos:end], end
}

// userAgentLike reports google's KeyIsUserAgent classification (robots.cc):
// key starts, case-insensitively, with "user-agent" or the tolerated typos
// "useragent" / "user agent". other_key rejects these so an extension line
// can never swallow a group boundary; the typo forms are deliberately left
// unparseable for now (docs/TODO.md "Malformed-input handling").
func userAgentLike(key string) bool {
	for _, p := range []string{"user-agent", "useragent", "user agent"} {
		if len(key) >= len(p) && strings.EqualFold(key[:len(p)], p) {
			return true
		}
	}
	return false
}

// matchOtherKey matches the key of an unknown "key : value" directive: a
// non-empty WS-trimmed run of (UTF8-char-noctl / WS) stopping before ":".
// ":" (0x3A) is inside UTF8-char-noctl's %x24-7F, so it must be excluded
// explicitly here; "#" is already outside the class (it starts a comment).
// Keys that are user-agent-like are rejected (group structure preservation).
func matchOtherKey(src string, pos int) (string, int) {
	text, end := keyValueRun(src, pos, ":")
	if end < 0 || userAgentLike(text) {
		return "", -1
	}
	return text, end
}

// matchAnyValue matches the value of a sitemap/unknown line: a non-empty
// WS-trimmed run of (UTF8-char-noctl / WS). "#" ends the run (it starts the
// trailing comment), as does the newline; both are outside the class.
func matchAnyValue(src string, pos int) (string, int) {
	return keyValueRun(src, pos, "")
}
