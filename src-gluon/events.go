package robotsgluon

// events.go — the "compiler" from the gluon CST to the deserialized form
// google's parser produces. src-google/robots.cc parses robots.txt into a
// stream of RobotsParseHandler callbacks — HandleUserAgent / HandleAllow /
// HandleDisallow / HandleSitemap / HandleUnknownAction, each carrying a line
// number and value. That callback stream IS google's deserialized
// representation, so it is the surface on which the two parsers are compared
// (tools/robots-dump prints it for the C++ side; Events computes it from the
// CST). The only robots.cc logic re-implemented here is per-line key
// classification (GetKeyType) and value canonicalization (MaybeEscapePattern);
// line splitting, comment stripping and whitespace trimming all happen
// structurally in the grammar.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/accretional/gluon/v2/metaparser"
	gluonpb "github.com/accretional/gluon/v2/pb"
)

// EventKind mirrors robots.cc KeyType.
type EventKind string

const (
	UserAgent EventKind = "USER_AGENT"
	Allow     EventKind = "ALLOW"
	Disallow  EventKind = "DISALLOW"
	Sitemap   EventKind = "SITEMAP"
	Unknown   EventKind = "UNKNOWN"
)

// Event is one RobotsParseHandler callback: robots.cc
// EmitKeyValueToHandler(line, key_type, key, value, handler). Key is only
// meaningful for Unknown events (HandleUnknownAction receives it; the typed
// handlers do not).
type Event struct {
	Line  int32
	Kind  EventKind
	Key   string
	Value string
}

func (e Event) String() string {
	if e.Kind == Unknown {
		return fmt.Sprintf("%d %s %q %q", e.Line, e.Kind, e.Key, e.Value)
	}
	return fmt.Sprintf("%d %s %q", e.Line, e.Kind, e.Value)
}

// Events parses src (see Parse) and lowers the CST to google's event stream.
func (g *Grammar) Events(src []byte) ([]Event, error) {
	norm := Normalize(src)
	ast, err := g.parseNormalized(norm)
	if err != nil {
		return nil, err
	}
	return compile(ast, norm)
}

// parseNormalized matches already-normalized text against the grammar.
func (g *Grammar) parseNormalized(norm []byte) (*gluonpb.ASTDescriptor, error) {
	doc := metaparser.WrapString(string(norm))
	doc.Name = "robots.txt"
	ast, err := metaparser.ParseCSTWithOptions(&gluonpb.CstRequest{
		Grammar:  g.gd,
		Document: doc,
	}, g.opts)
	if err != nil {
		return nil, fmt.Errorf("robots.txt does not match RFC 9309 grammar: %w", err)
	}
	return ast, nil
}

// compile walks the CST in source order and emits one Event per directive
// line, with line numbers computed the way robots.cc counts lines.
func compile(ast *gluonpb.ASTDescriptor, norm []byte) ([]Event, error) {
	lines := newLineIndex(norm)
	var events []Event
	var walk func(n *gluonpb.ASTNode) error
	walk = func(n *gluonpb.ASTNode) error {
		if n == nil {
			return nil
		}
		switch n.GetKind() {
		case "startgroupline", "rule", "sitemapline", "otherline":
			ev, err := lineEvent(n, lines)
			if err != nil {
				return err
			}
			events = append(events, ev)
			return nil
		}
		for _, c := range n.GetChildren() {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(ast.GetRoot()); err != nil {
		return nil, err
	}
	return events, nil
}

// lineEvent lowers one directive-line CST node to its handler callback,
// resolving the line number from the node's offset.
func lineEvent(n *gluonpb.ASTNode, lines *lineIndex) (Event, error) {
	return lineEventAt(n, lines.at(n.GetLocation().GetOffset()))
}

// lineEventAt lowers a directive-line CST node with a known line number —
// the recovery path (recover.go) parses one physical line at a time, so
// node offsets are line-local and the caller supplies the real line number.
func lineEventAt(n *gluonpb.ASTNode, line int32) (Event, error) {
	switch n.GetKind() {
	case "startgroupline":
		// HandleUserAgent(line, product-token). Never escaped (robots.cc
		// NeedEscapeValueForKey: USER_AGENT/SITEMAP are exempt).
		return Event{Line: line, Kind: UserAgent, Value: subtreeText(find(n, "product_token"))}, nil
	case "rule":
		key := subtreeText(find(n, "rule_key"))
		kind := Allow
		if strings.EqualFold(key, "disallow") {
			kind = Disallow
		}
		return Event{Line: line, Kind: kind, Value: escapePattern(subtreeText(find(n, "path_pattern")))}, nil
	case "sitemapline":
		return Event{Line: line, Kind: Sitemap, Value: subtreeText(find(n, "any_value"))}, nil
	case "otherline":
		key := subtreeText(find(n, "other_key"))
		value := subtreeText(find(n, "any_value"))
		switch classifyKey(key) {
		case Allow:
			return Event{Line: line, Kind: Allow, Value: escapePattern(value)}, nil
		case Disallow:
			return Event{Line: line, Kind: Disallow, Value: escapePattern(value)}, nil
		case Sitemap:
			return Event{Line: line, Kind: Sitemap, Value: value}, nil
		default:
			// HandleUnknownAction(line, key, value); value is escaped
			// (NeedEscapeValueForKey defaults to true).
			return Event{Line: line, Kind: Unknown, Key: key, Value: escapePattern(value)}, nil
		}
	}
	return Event{}, fmt.Errorf("not a directive line node: %q", n.GetKind())
}

// classifyKey mirrors robots.cc GetKeyType's order and PREFIX matching
// (absl::StartsWithIgnoreCase), including the kAllowFrequentTypos spellings.
// The user-agent arm is unreachable from otherline (other_key rejects
// user-agent-like keys) but kept for fidelity and reuse.
func classifyKey(key string) EventKind {
	switch {
	case hasFoldPrefix(key, "user-agent"), hasFoldPrefix(key, "useragent"), hasFoldPrefix(key, "user agent"):
		return UserAgent
	case hasFoldPrefix(key, "allow"):
		return Allow
	case hasFoldPrefix(key, "disallow"),
		hasFoldPrefix(key, "dissallow"), hasFoldPrefix(key, "dissalow"),
		hasFoldPrefix(key, "disalow"), hasFoldPrefix(key, "diasllow"),
		hasFoldPrefix(key, "disallaw"):
		return Disallow
	case hasFoldPrefix(key, "sitemap"), hasFoldPrefix(key, "site-map"):
		return Sitemap
	default:
		return Unknown
	}
}

func hasFoldPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// escapePattern is a byte-exact port of robots.cc MaybeEscapePattern:
// %-escape sequences get their hex digits uppercased, bytes with the high
// bit set get %-escaped (uppercase hex), everything else is copied.
func escapePattern(src string) string {
	needsWork := false
	for i := 0; i < len(src); i++ {
		if src[i] == '%' && i+2 < len(src) && isHex(src[i+1]) && isHex(src[i+2]) {
			if isLowerHex(src[i+1]) || isLowerHex(src[i+2]) {
				needsWork = true
			}
			i += 2
		} else if src[i]&0x80 != 0 {
			needsWork = true
		}
	}
	if !needsWork {
		return src
	}
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		switch {
		case src[i] == '%' && i+2 < len(src) && isHex(src[i+1]) && isHex(src[i+2]):
			b.WriteByte('%')
			b.WriteByte(upperHex(src[i+1]))
			b.WriteByte(upperHex(src[i+2]))
			i += 2
		case src[i]&0x80 != 0:
			b.WriteByte('%')
			b.WriteByte(hexDigits[src[i]>>4&0xF])
			b.WriteByte(hexDigits[src[i]&0xF])
		default:
			b.WriteByte(src[i])
		}
	}
	return b.String()
}

func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
func isLowerHex(b byte) bool { return b >= 'a' && b <= 'f' }
func upperHex(b byte) byte {
	if isLowerHex(b) {
		return b - 'a' + 'A'
	}
	return b
}

// find returns the first node of the given kind in a depth-first walk, or
// nil. Directive lines never nest, so this is unambiguous within one line.
func find(n *gluonpb.ASTNode, kind string) *gluonpb.ASTNode {
	if n == nil {
		return nil
	}
	if n.GetKind() == kind {
		return n
	}
	for _, c := range n.GetChildren() {
		if hit := find(c, kind); hit != nil {
			return hit
		}
	}
	return nil
}

// subtreeText concatenates, in source order, the values of all leaf nodes
// under n — terminals and matcher tokens both carry their matched text in
// Value; structural nodes have children and empty values. nil-safe ("").
func subtreeText(n *gluonpb.ASTNode) string {
	var b strings.Builder
	var walk func(*gluonpb.ASTNode)
	walk = func(n *gluonpb.ASTNode) {
		if n == nil {
			return
		}
		if len(n.GetChildren()) == 0 {
			b.WriteString(n.GetValue())
			return
		}
		for _, c := range n.GetChildren() {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// lineIndex maps byte offsets in normalized source to robots.cc line
// numbers: every CR ends a line, every LF not immediately preceded by CR
// ends a line (so CRLF counts once), numbering is 1-based.
type lineIndex struct{ breaks []int }

func newLineIndex(src []byte) *lineIndex {
	var breaks []int
	for i, b := range src {
		switch b {
		case '\r':
			breaks = append(breaks, i)
		case '\n':
			if i == 0 || src[i-1] != '\r' {
				breaks = append(breaks, i)
			}
		}
	}
	return &lineIndex{breaks: breaks}
}

func (li *lineIndex) at(offset int32) int32 {
	n := sort.SearchInts(li.breaks, int(offset))
	return int32(n) + 1
}
