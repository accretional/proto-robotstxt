package robotsgluon

// recover.go — tier 2 of the two-tier parse (docs/design/two-tier-parsing.md;
// robots.txt instance: docs/design/malformed-input.md). When the strict
// whole-document parse fails, the input is split into physical lines exactly
// the way google's parser splits them, each line is re-tried against the
// SAME grammar's line rules (via gluon's ParseOptions.StartRule), and lines
// no rule matches fall back to a port of robots.cc GetKeyAndValueFrom. The
// BNF core is never loosened: strict lines keep their grammar parse, and
// every deviation is reported as an IrregularLine with a reason.

import (
	"fmt"
	"strings"

	"github.com/accretional/gluon/v2/metaparser"
	gluonpb "github.com/accretional/gluon/v2/pb"
)

// lineRules are the grammar's per-line rules, tried in order. RFC-core
// rules come first (mirroring the alternation order inside the grammar);
// emptyline last since it only matches blank/comment-only lines.
var lineRules = []string{"startgroupline", "rule", "sitemapline", "otherline", "emptyline"}

// LineResult records how one physical line parsed during recovery.
type LineResult struct {
	Line int32  // 1-based, google numbering (CR, LF, CRLF each end one line)
	Text string // the line's bytes, terminator excluded
	Rule string // grammar rule that matched, or "" when Irregular
	// Irregular means no grammar rule matched and the robots.cc fallback
	// extraction was used. Reason says what the fallback concluded.
	Irregular bool
	Reason    string
}

// Recovered is the result of the two-tier parse.
type Recovered struct {
	// Strict is the whole-document CST when tier 1 succeeded, else nil.
	Strict *gluonpb.ASTDescriptor
	// Lines is the per-line record (tier 2 only; nil when Strict != nil).
	Lines []LineResult
	// Events is google's deserialization of the document, from whichever
	// tier ran. This is the stream `gluon check -recover` diffs against
	// tools/robots-dump.
	Events []Event
}

// Recover parses src with the two-tier strategy. It fails only on internal
// errors — every byte sequence yields a Recovered (like google's parser,
// which accepts any input and reports what it could extract).
func (g *Grammar) Recover(src []byte) (*Recovered, error) {
	norm := Normalize(src)
	if ast, err := g.parseNormalized(norm); err == nil {
		events, err := compile(ast, norm)
		if err != nil {
			return nil, err
		}
		return &Recovered{Strict: ast, Events: events}, nil
	}

	var rec Recovered
	for _, seg := range splitPhysicalLines(norm) {
		lr, ev, err := g.recoverLine(seg)
		if err != nil {
			return nil, err
		}
		rec.Lines = append(rec.Lines, lr)
		if ev != nil {
			rec.Events = append(rec.Events, *ev)
		}
	}
	return &rec, nil
}

// lineSegment is one physical line: text excludes the terminator, raw
// includes it (the grammar's line rules require the trailing NL).
type lineSegment struct {
	num  int32
	text string
	raw  string
}

// splitPhysicalLines splits normalized input exactly like robots.cc Parse():
// every 0x0A or 0x0D ends a line, except an LF immediately following a CR
// (one CRLF ending). The final empty segment after a trailing terminator is
// dropped — google does emit it (metadata-only), but it can never carry a
// directive, and recovery reports lines, not EOF bookkeeping.
func splitPhysicalLines(norm []byte) []lineSegment {
	var segs []lineSegment
	var num int32 = 1
	start := 0
	for i := 0; i < len(norm); i++ {
		b := norm[i]
		if b != '\n' && b != '\r' {
			continue
		}
		end := i + 1
		if b == '\r' && i+1 < len(norm) && norm[i+1] == '\n' {
			end = i + 2
		}
		segs = append(segs, lineSegment{
			num:  num,
			text: string(norm[start:i]),
			raw:  string(norm[start:end]),
		})
		num++
		i = end - 1
		start = end
	}
	if start < len(norm) {
		// Unterminated tail — unreachable after Normalize, but keep the
		// splitter total.
		segs = append(segs, lineSegment{num: num, text: string(norm[start:]), raw: string(norm[start:])})
	}
	return segs
}

// recoverLine classifies one physical line: strict grammar line rules
// first, robots.cc-equivalent fallback otherwise.
func (g *Grammar) recoverLine(seg lineSegment) (LineResult, *Event, error) {
	for _, rule := range lineRules {
		node, err := g.parseLineAs(seg.raw, rule)
		if err != nil {
			continue
		}
		lr := LineResult{Line: seg.num, Text: seg.text, Rule: rule}
		if rule == "emptyline" {
			return lr, nil, nil
		}
		ev, err := lineEventAt(node, seg.num)
		if err != nil {
			return lr, nil, err
		}
		return lr, &ev, nil
	}

	key, value, reason := extractIrregular(seg.text)
	lr := LineResult{Line: seg.num, Text: seg.text, Irregular: true, Reason: reason}
	if key == "" {
		return lr, nil, nil // no directive (junk, empty-after-comment, ...)
	}
	ev := irregularEvent(seg.num, key, value)
	return lr, &ev, nil
}

// parseLineAs matches one raw line (terminator included) against a single
// grammar rule, whole-line consumption required.
func (g *Grammar) parseLineAs(raw string, rule string) (*gluonpb.ASTNode, error) {
	doc := metaparser.WrapString(raw)
	opts := &metaparser.ParseOptions{
		TokenMatchers:       g.opts.TokenMatchers,
		DisableAutoComments: g.opts.DisableAutoComments,
		StartRule:           rule,
	}
	ast, err := metaparser.ParseCSTWithOptions(&gluonpb.CstRequest{
		Grammar:  g.gd,
		Document: doc,
	}, opts)
	if err != nil {
		return nil, err
	}
	return ast.GetRoot(), nil
}

// extractIrregular ports robots.cc GetKeyAndValueFrom byte-for-byte for a
// single line (terminator already excluded). Returns key == "" when the
// line carries no directive; reason explains the outcome either way.
func extractIrregular(line string) (key, value, reason string) {
	// C string semantics: everything past the first NUL is invisible to
	// robots.cc's strchr/strlen-based scanning.
	if i := strings.IndexByte(line, 0x00); i >= 0 {
		line = line[:i]
	}
	// Comment strip, then ASCII whitespace trim (absl::StripAsciiWhitespace).
	hadComment := false
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
		hadComment = true
	}
	line = strings.Trim(line, " \t\n\v\f\r")
	if line == "" {
		if hadComment {
			return "", "", "comment-only"
		}
		return "", "", "empty"
	}

	sep := strings.IndexByte(line, ':')
	if sep < 0 {
		// Google-specific: accept whitespace for a forgotten colon, but only
		// when the line is exactly two runs of non-whitespace.
		ws := strings.IndexAny(line, " \t")
		if ws < 0 {
			return "", "", "no-separator"
		}
		val := strings.TrimLeft(line[ws:], " \t")
		if strings.ContainsAny(val, " \t") {
			return "", "", "no-separator-multi-token"
		}
		return strings.TrimRight(line[:ws], " \t\n\v\f\r"), val, "missing-colon-separator"
	}

	key = strings.Trim(line[:sep], " \t\n\v\f\r")
	if key == "" {
		return "", "", "empty-key"
	}
	value = strings.Trim(line[sep+1:], " \t\n\v\f\r")
	return key, value, "directive-outside-grammar"
}

// irregularEvent classifies an extracted key/value pair exactly as
// robots.cc ParseAndEmitLine does (GetKeyType + NeedEscapeValueForKey).
func irregularEvent(line int32, key, value string) Event {
	switch classifyKey(key) {
	case UserAgent:
		return Event{Line: line, Kind: UserAgent, Value: value}
	case Allow:
		return Event{Line: line, Kind: Allow, Value: escapePattern(value)}
	case Disallow:
		return Event{Line: line, Kind: Disallow, Value: escapePattern(value)}
	case Sitemap:
		return Event{Line: line, Kind: Sitemap, Value: value}
	default:
		return Event{Line: line, Kind: Unknown, Key: key, Value: escapePattern(value)}
	}
}

// RecoverSummary renders a short human-readable account of a recovery for
// CLI/diagnostic output.
func (r *Recovered) RecoverSummary() string {
	if r.Strict != nil {
		return "strict (tier 1)"
	}
	irregular := 0
	for _, l := range r.Lines {
		if l.Irregular {
			irregular++
		}
	}
	return fmt.Sprintf("recovered (tier 2): %d line(s), %d irregular", len(r.Lines), irregular)
}
