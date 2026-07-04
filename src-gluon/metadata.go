package robotsgluon

// metadata.go — the google-exact physical-line scanner. One pass computes,
// for every input (both tiers), exactly what robots.cc computes line-
// locally: the line split (BOM skip, CR/LF/CRLF, always-emitted final
// segment), the per-line length cap (kMaxLineLen), and each line's
// RobotsParseHandler::LineMetadata plus extracted directive. Recovery
// (recover.go) reuses the same scan for its fallback so metadata and events
// can never disagree about a line.

import (
	"strconv"
	"strings"
)

// maxLineLen mirrors robots.cc kMaxLineLen (2083 * 8). Google's line buffer
// holds at most maxLineLen-1 content bytes; the rest of the line is dropped
// and the line is flagged too-long.
const maxLineLen = 2083 * 8

// LineMetadata mirrors RobotsParseHandler::LineMetadata (robots.h), plus
// the line number it is reported for.
type LineMetadata struct {
	Line                    int32
	IsEmpty                 bool
	HasComment              bool
	IsComment               bool
	HasDirective            bool
	IsAcceptableTypo        bool
	IsLineTooLong           bool
	IsMissingColonSeparator bool
}

// googleLine is one physical line as robots.cc sees it.
type googleLine struct {
	num     int32
	text    string // content, terminator excluded, truncated to maxLineLen-1
	tooLong bool
	final   bool // the always-emitted segment after the last terminator
}

// googleLines splits raw input exactly like robots.cc Parse(): the longest
// matching UTF-8 BOM prefix is consumed; every 0x0A or 0x0D ends a line
// except an LF immediately after a CR (one CRLF ending); content beyond
// maxLineLen-1 bytes is dropped and flags the line; and the segment after
// the last terminator is always emitted, even when empty (google's EOF
// flush) — marked final.
func googleLines(src []byte) []googleLine {
	bom := []byte{0xEF, 0xBB, 0xBF}
	i := 0
	for i < len(bom) && i < len(src) && src[i] == bom[i] {
		i++
	}
	src = src[i:]

	var lines []googleLine
	var num int32 = 1
	start := 0
	emit := func(end int, final bool) {
		text := src[start:end]
		tooLong := len(text) > maxLineLen-1
		if tooLong {
			text = text[:maxLineLen-1]
		}
		lines = append(lines, googleLine{num: num, text: string(text), tooLong: tooLong, final: final})
		num++
	}
	for j := 0; j < len(src); j++ {
		b := src[j]
		if b != '\n' && b != '\r' {
			continue
		}
		emit(j, false)
		if b == '\r' && j+1 < len(src) && src[j+1] == '\n' {
			j++
		}
		start = j + 1
	}
	emit(len(src), true)
	return lines
}

// lineDirective is everything robots.cc derives from one line:
// GetKeyAndValueFrom + GetKeyType, metadata flags included. Key == ""
// means the line carries no directive.
type lineDirective struct {
	key, value string
	kind       EventKind
	meta       LineMetadata // Line/IsLineTooLong filled by the caller
	reason     string       // human-readable why (LineResult.Reason)
}

// parseGoogleLine ports robots.cc GetKeyAndValueFrom + GetKeyType for one
// line (terminator excluded, already length-capped).
func parseGoogleLine(text string) lineDirective {
	var d lineDirective
	line := text
	// C string semantics: everything past the first NUL is invisible.
	if i := strings.IndexByte(line, 0x00); i >= 0 {
		line = line[:i]
	}
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
		d.meta.HasComment = true
	}
	line = strings.Trim(line, " \t\n\v\f\r")
	if line == "" {
		if d.meta.HasComment {
			d.meta.IsComment = true
			d.reason = "comment-only"
		} else {
			d.meta.IsEmpty = true
			d.reason = "empty"
		}
		return d
	}

	sep := strings.IndexByte(line, ':')
	if sep < 0 {
		// Google-specific: accept whitespace for a forgotten colon, but only
		// when the line is exactly two runs of non-whitespace.
		ws := strings.IndexAny(line, " \t")
		if ws < 0 {
			d.reason = "no-separator"
			return d
		}
		val := strings.TrimLeft(line[ws:], " \t")
		if strings.ContainsAny(val, " \t") {
			d.reason = "no-separator-multi-token"
			return d
		}
		d.meta.IsMissingColonSeparator = true
		d.key = strings.TrimRight(line[:ws], " \t\n\v\f\r")
		d.value = val
		d.reason = "missing-colon-separator"
	} else {
		d.key = strings.Trim(line[:sep], " \t\n\v\f\r")
		if d.key == "" {
			d.reason = "empty-key"
			return d
		}
		d.value = strings.Trim(line[sep+1:], " \t\n\v\f\r")
		d.reason = "directive-outside-grammar"
	}
	d.meta.HasDirective = true
	d.kind, d.meta.IsAcceptableTypo = classifyKeyTypo(d.key)
	return d
}

// classifyKeyTypo mirrors robots.cc GetKeyType exactly, including which
// prefix matches count as "acceptable typos" (kAllowFrequentTypos). Order
// and short-circuiting match the C++ (user-agent, allow, disallow, sitemap).
func classifyKeyTypo(key string) (EventKind, bool) {
	switch {
	case hasFoldPrefix(key, "user-agent"):
		return UserAgent, hasFoldPrefix(key, "useragent") || hasFoldPrefix(key, "user agent")
	case hasFoldPrefix(key, "useragent"), hasFoldPrefix(key, "user agent"):
		return UserAgent, true
	case hasFoldPrefix(key, "allow"):
		return Allow, false
	case hasFoldPrefix(key, "disallow"):
		return Disallow, isDisallowTypo(key)
	case isDisallowTypo(key):
		return Disallow, true
	case hasFoldPrefix(key, "sitemap"):
		return Sitemap, hasFoldPrefix(key, "site-map")
	case hasFoldPrefix(key, "site-map"):
		return Sitemap, true
	default:
		return Unknown, false
	}
}

func isDisallowTypo(key string) bool {
	for _, p := range []string{"dissallow", "dissalow", "disalow", "diasllow", "disallaw"} {
		if hasFoldPrefix(key, p) {
			return true
		}
	}
	return false
}

// LineMetadataOf computes google's ReportLineMetadata stream for raw input:
// one record per physical line, in order, both spec-valid and not. This is
// the second comparison surface next to the event stream.
func LineMetadataOf(src []byte) []LineMetadata {
	lines := googleLines(src)
	out := make([]LineMetadata, 0, len(lines))
	for _, ln := range lines {
		d := parseGoogleLine(ln.text)
		m := d.meta
		m.Line = ln.num
		m.IsLineTooLong = ln.tooLong
		out = append(out, m)
	}
	return out
}

// DiffMetadata compares two metadata streams, returning human-readable
// differences (empty = equal).
func DiffMetadata(gluon, google []LineMetadata) []string {
	var diffs []string
	n := max(len(gluon), len(google))
	for i := 0; i < n; i++ {
		switch {
		case i >= len(gluon):
			diffs = append(diffs, "meta "+strconv.Itoa(i)+": gluon <none> | google "+google[i].String())
		case i >= len(google):
			diffs = append(diffs, "meta "+strconv.Itoa(i)+": gluon "+gluon[i].String()+" | google <none>")
		default:
			if gluon[i] != google[i] {
				diffs = append(diffs, "meta "+strconv.Itoa(i)+": gluon "+gluon[i].String()+" | google "+google[i].String())
			}
		}
	}
	return diffs
}

func (m LineMetadata) String() string {
	flags := ""
	for _, f := range []struct {
		name string
		on   bool
	}{
		{"empty", m.IsEmpty}, {"has-comment", m.HasComment}, {"comment", m.IsComment},
		{"directive", m.HasDirective}, {"typo", m.IsAcceptableTypo},
		{"too-long", m.IsLineTooLong}, {"missing-colon", m.IsMissingColonSeparator},
	} {
		if f.on {
			if flags != "" {
				flags += ","
			}
			flags += f.name
		}
	}
	if flags == "" {
		flags = "-"
	}
	return "{line " + strconv.Itoa(int(m.Line)) + " " + flags + "}"
}
