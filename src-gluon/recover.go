package robotsgluon

// recover.go — tier 2 of the two-tier parse (docs/design/two-tier-parsing.md;
// robots.txt instance: docs/design/malformed-input.md). When the strict
// whole-document parse fails — or google's per-line length cap makes strict
// semantics diverge from google's — the input is split into physical lines
// exactly the way google's parser splits them (metadata.go), each line is
// re-tried against the SAME grammar's line rules (gluon
// ParseOptions.StartRule), and lines no rule matches fall back to the
// robots.cc GetKeyAndValueFrom port. The BNF core is never loosened: strict
// lines keep their grammar parse, and every deviation is reported as an
// irregular LineResult with a reason. Metadata (google's ReportLineMetadata
// stream) is computed for every input, both tiers.

import (
	"github.com/accretional/gluon/v2/metaparser"
	gluonpb "github.com/accretional/gluon/v2/pb"

	"fmt"
)

// lineRules are the grammar's per-line rules, tried in order. RFC-core
// rules come first (mirroring the alternation order inside the grammar);
// emptyline last since it only matches blank/comment-only lines.
var lineRules = []string{"startgroupline", "rule", "sitemapline", "otherline", "emptyline"}

// LineResult records how one physical line parsed during recovery.
type LineResult struct {
	Line int32  // 1-based, google numbering (CR, LF, CRLF each end one line)
	Text string // the line's bytes, terminator excluded, length-capped
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
	// Google's phantom empty EOF segment is excluded — it appears only in
	// Metadata, which mirrors google's line accounting exactly.
	Lines []LineResult
	// Events is google's deserialization of the document, from whichever
	// tier ran. `gluon check -recover` diffs this against tools/robots-dump.
	Events []Event
	// Metadata is google's ReportLineMetadata stream (one record per
	// physical line including the EOF segment), computed for BOTH tiers —
	// it is a pure line-local function of the input.
	Metadata []LineMetadata
}

// Recover parses src with the two-tier strategy. It fails only on internal
// errors — every byte sequence yields a Recovered (like google's parser,
// which accepts any input and reports what it could extract).
func (g *Grammar) Recover(src []byte) (*Recovered, error) {
	lines := googleLines(src)

	metadata := make([]LineMetadata, 0, len(lines))
	tooLong := false
	directives := make([]lineDirective, len(lines))
	for i, ln := range lines {
		directives[i] = parseGoogleLine(ln.text)
		m := directives[i].meta
		m.Line = ln.num
		m.IsLineTooLong = ln.tooLong
		metadata = append(metadata, m)
		tooLong = tooLong || ln.tooLong
	}

	// Tier 1 — unless a line exceeded google's cap: google then parses
	// TRUNCATED content, so even a spec-valid document would deserialize
	// differently; recovery's per-line path applies the same truncation.
	if !tooLong {
		norm := Normalize(src)
		if ast, err := g.parseNormalized(norm); err == nil {
			events, err := compile(ast, norm)
			if err != nil {
				return nil, err
			}
			return &Recovered{Strict: ast, Events: events, Metadata: metadata}, nil
		}
	}

	rec := &Recovered{Metadata: metadata}
	for i, ln := range lines {
		if ln.final && ln.text == "" {
			continue // google's EOF flush: metadata-only, never a directive
		}
		lr, ev, err := g.recoverLine(ln, directives[i])
		if err != nil {
			return nil, err
		}
		rec.Lines = append(rec.Lines, lr)
		if ev != nil {
			rec.Events = append(rec.Events, *ev)
		}
	}
	return rec, nil
}

// recoverLine classifies one physical line: strict grammar line rules
// first, the pre-computed robots.cc fallback otherwise.
func (g *Grammar) recoverLine(ln googleLine, d lineDirective) (LineResult, *Event, error) {
	for _, rule := range lineRules {
		node, err := g.parseLineAs(ln.text+"\n", rule)
		if err != nil {
			continue
		}
		lr := LineResult{Line: ln.num, Text: ln.text, Rule: rule}
		if rule == "emptyline" {
			return lr, nil, nil
		}
		ev, err := lineEventAt(node, ln.num)
		if err != nil {
			return lr, nil, err
		}
		return lr, &ev, nil
	}

	lr := LineResult{Line: ln.num, Text: ln.text, Irregular: true, Reason: d.reason}
	if d.key == "" {
		return lr, nil, nil // no directive (junk, empty-after-comment, ...)
	}
	ev := directiveEvent(ln.num, d)
	return lr, &ev, nil
}

// parseLineAs matches one line (a trailing NL appended — the grammar's line
// rules end in eol, and google treats both EOL and EOF as line ends)
// against a single grammar rule, whole-line consumption required.
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

// directiveEvent turns an extracted irregular directive into its handler
// event exactly as robots.cc ParseAndEmitLine does (NeedEscapeValueForKey:
// USER_AGENT and SITEMAP values are passed through, the rest get
// MaybeEscapePattern).
func directiveEvent(line int32, d lineDirective) Event {
	switch d.kind {
	case UserAgent, Sitemap:
		return Event{Line: line, Kind: d.kind, Value: d.value}
	case Unknown:
		return Event{Line: line, Kind: Unknown, Key: d.key, Value: escapePattern(d.value)}
	default: // Allow, Disallow
		return Event{Line: line, Kind: d.kind, Value: escapePattern(d.value)}
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
