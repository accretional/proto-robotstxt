// Package robotsgluon parses robots.txt with a grammar-driven parser: the
// RFC 9309 ABNF, formalized as EBNF in grammar/rep.ebnf, is loaded through
// gluon v2's metaparser and matched against documents to produce a full
// concrete syntax tree — no hand-written robots.txt parsing. A small
// "compiler" (events.go) then lowers the CST to the same deserialized form
// google's C++ parser (src-google/robots.cc) hands to its RobotsParseHandler,
// which is what the cross-parser tests compare. See README.md in this
// directory.
package robotsgluon

import (
	"fmt"
	"os"

	"github.com/accretional/gluon/v2/metaparser"
	gluonpb "github.com/accretional/gluon/v2/pb"

	"github.com/accretional/proto-robotstxt/grammar"
)

// Grammar is a loaded, parse-ready robots.txt grammar: the GrammarDescriptor
// from grammar/rep.ebnf with whitespace significance enabled and the token
// matchers from matchers.go attached.
type Grammar struct {
	gd   *gluonpb.GrammarDescriptor
	opts *metaparser.ParseOptions
}

// Default loads the embedded grammar/rep.ebnf.
func Default() (*Grammar, error) {
	return fromEBNF(grammar.RepEBNF, "rep.ebnf")
}

// LoadGrammar loads a grammar from an explicit .ebnf path (same dialect and
// matcher contract as grammar/rep.ebnf).
func LoadGrammar(path string) (*Grammar, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return fromEBNF(src, path)
}

func fromEBNF(src []byte, name string) (*Grammar, error) {
	doc := metaparser.WrapString(string(src))
	doc.Name = name
	gd, err := metaparser.ParseEBNF(doc)
	if err != nil {
		return nil, fmt.Errorf("parse EBNF %s: %w", name, err)
	}
	if len(gd.GetRules()) == 0 {
		return nil, fmt.Errorf("parse EBNF %s: no rules", name)
	}
	if first := gd.GetRules()[0].GetName(); first != "robotstxt" {
		// The metaparser hard-codes rule 0 as the start rule.
		return nil, fmt.Errorf("parse EBNF %s: first rule is %q, want robotstxt", name, first)
	}

	// robots.txt is line-oriented: strip the stock EBNF WHITESPACE symbols so
	// nothing is skipped implicitly and the grammar's explicit ws/nl tokens
	// see every byte (metaparser derives its skip-set from the lex).
	stripWhitespace(gd)

	return &Grammar{
		gd: gd,
		opts: &metaparser.ParseOptions{
			TokenMatchers: Matchers(),
			// robots.txt has no //, /*, (* comment syntax — "#"-comments are
			// grammar rules. Without this, a "//"-prefixed path could be
			// silently skipped as a line comment in syntactic contexts.
			DisableAutoComments: true,
		},
	}, nil
}

// stripWhitespace removes WHITESPACE delimiter symbols from the grammar's lex.
func stripWhitespace(gd *gluonpb.GrammarDescriptor) {
	lex := gd.GetLex()
	if lex == nil {
		return
	}
	kept := lex.GetSymbols()[:0]
	for _, sym := range lex.GetSymbols() {
		if d := sym.GetDelimiter(); d != nil && d.GetKind() == gluonpb.Delimiter_WHITESPACE {
			continue
		}
		kept = append(kept, sym)
	}
	lex.Symbols = kept
}

// Normalize applies the two canonicalizations google's parser performs on
// every input before line-parsing (robots.cc RobotsTxtParser::Parse), so the
// strict grammar sees the same effective document:
//
//  1. a UTF-8 BOM prefix is stripped — google skips the longest matching
//     prefix of EF BB BF, even a partial one (its bom_pos scan consumes
//     matching bytes as they arrive);
//  2. a final newline is appended when non-empty input does not end in CR or
//     LF — google's parser emits the final unterminated line at EOF.
//
// Offsets/line numbers reported from a parse refer to the normalized text;
// BOM stripping removes whole leading bytes only and the appended byte is at
// EOF, so line numbers are unchanged from the original.
func Normalize(src []byte) []byte {
	bom := []byte{0xEF, 0xBB, 0xBF}
	i := 0
	for i < len(bom) && i < len(src) && src[i] == bom[i] {
		i++
	}
	src = src[i:]
	if len(src) > 0 && src[len(src)-1] != '\n' && src[len(src)-1] != '\r' {
		out := make([]byte, len(src)+1)
		copy(out, src)
		out[len(src)] = '\n'
		return out
	}
	return src
}

// Parse normalizes src (see Normalize) and matches it against the grammar,
// returning the full concrete syntax tree. The whole input must parse; any
// unconsumed suffix is an error (that is the strict-RFC posture — google-only
// leniencies are listed at the top of grammar/rep.ebnf and in docs/TODO.md).
func (g *Grammar) Parse(src []byte) (*gluonpb.ASTDescriptor, error) {
	doc := metaparser.WrapString(string(Normalize(src)))
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
