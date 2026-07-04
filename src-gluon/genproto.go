package robotsgluon

// genproto.go — derives the typed proto representation of robots.txt from
// grammar/rep.ebnf via gluon v2's compiler (the kvq/proto-sqlite genproto
// pipeline, adapted): ParseEBNF → GrammarToAST → typedRepAST transform →
// Compile → FileDescriptorProto. cmd/gluon writes the results to
// gen/rep.{proto,fdset} (git-ignored); the consolidated, checked-in copy is
// proto/rep.proto.
//
// The typedRepAST transform maps the parse grammar onto a schema-shaped AST:
//
//   - structural noise carries no data and is dropped: ws/nl/eol/comment/
//     comment_char/emptyline, the ":" separator terminals, and the constant
//     line keys (useragent_key, sitemap_key);
//   - lexical value rules become proto3 string scalars: product_token,
//     path_pattern, other_key, any_value (their character-level sub-rules
//     identifier/id_char/utf8_char_noctl go with them);
//   - allow_key/disallow_key become keyword terminals "allow"/"disallow" so
//     rule_key lowers to a oneof over empty marker messages — the
//     allow-vs-disallow distinction stays in the schema.

import (
	"fmt"
	"os"
	"strings"

	"github.com/jhump/protoreflect/v2/protoprint"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/accretional/gluon/v2/compiler"
	"github.com/accretional/gluon/v2/metaparser"
	gluonpb "github.com/accretional/gluon/v2/pb"

	"github.com/accretional/proto-robotstxt/grammar"
)

// GrammarDescriptor parses the grammar (path == "" for the embedded
// grammar/rep.ebnf) and returns the raw v2 GrammarDescriptor — the
// proto-message representation of the grammar itself.
func GrammarDescriptor(path string) (*gluonpb.GrammarDescriptor, error) {
	src := grammar.RepEBNF
	name := "rep.ebnf"
	if path != "" {
		var err error
		if src, err = os.ReadFile(path); err != nil {
			return nil, err
		}
		name = path
	}
	doc := metaparser.WrapString(string(src))
	doc.Name = name
	gd, err := metaparser.ParseEBNF(doc)
	if err != nil {
		return nil, fmt.Errorf("parse EBNF %s: %w", name, err)
	}
	return gd, nil
}

// GenprotoOptions configures Genproto.
type GenprotoOptions struct {
	Package   string // proto package name, e.g. "robotstxt.rep"
	GoPackage string // go_package option ("" = omit)
}

// GenprotoResult is the derived schema in both machine and human form. The
// descriptor set holds two files: rep.proto (grammar-derived) and
// recover.proto (hand-built two-tier shapes wrapping it — recoverproto.go).
type GenprotoResult struct {
	ProtoSrc        string // rendered rep.proto source
	RecoverProtoSrc string // rendered recover.proto source
	FdsetBytes      []byte // wire-format FileDescriptorSet (both files)
	Messages        int
}

// Genproto derives the typed proto schema from the grammar.
func Genproto(grammarPath string, opts GenprotoOptions) (*GenprotoResult, error) {
	gd, err := GrammarDescriptor(grammarPath)
	if err != nil {
		return nil, err
	}
	ast, err := compiler.GrammarToAST(gd)
	if err != nil {
		return nil, fmt.Errorf("GrammarToAST: %w", err)
	}
	ast.Root = typedRepAST(ast.Root)

	fdp, err := compiler.Compile(ast, compiler.Options{
		Package:   opts.Package,
		GoPackage: opts.GoPackage,
		FileName:  "rep.proto",
	})
	if err != nil {
		return nil, fmt.Errorf("compiler.Compile: %w", err)
	}

	recoverFdp := recoverFileDescriptor(opts.Package)
	set := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp, recoverFdp},
	}
	blob, err := proto.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("marshal fdset: %w", err)
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, fmt.Errorf("protodesc.NewFiles: %w", err)
	}
	src, err := renderProtoFile(files, "rep.proto")
	if err != nil {
		return nil, err
	}
	recoverSrc, err := renderProtoFile(files, "recover.proto")
	if err != nil {
		return nil, err
	}
	return &GenprotoResult{
		ProtoSrc:        src,
		RecoverProtoSrc: recoverSrc,
		FdsetBytes:      blob,
		Messages:        len(fdp.GetMessageType()) + len(recoverFdp.GetMessageType()),
	}, nil
}

// droppedRules are grammar rules that carry no data in the typed rep.
var droppedRules = map[string]bool{
	"ws": true, "nl": true, "eol": true,
	"comment": true, "comment_char": true, "emptyline": true,
	"identifier": true, "id_char": true, "utf8_char_noctl": true,
	"useragent_key": true, "sitemap_key": true,
	"allow_key": true, "disallow_key": true,
}

// scalarRules lower to proto3 string fields named after the rule.
var scalarRules = map[string]bool{
	"product_token": true, "path_pattern": true,
	"other_key": true, "any_value": true,
}

// keywordRules: references become keyword terminals so alternations over
// them stay distinguishable as oneof arms of empty marker messages.
var keywordRules = map[string]string{
	"allow_key":    "allow",
	"disallow_key": "disallow",
}

// typedRepAST rewrites the schema AST per the mapping documented at the top
// of this file.
func typedRepAST(root *gluonpb.ASTNode) *gluonpb.ASTNode {
	var rules []*gluonpb.ASTNode
	for _, r := range root.GetChildren() {
		name := r.GetValue()
		if droppedRules[name] || scalarRules[name] {
			continue
		}
		body := make([]*gluonpb.ASTNode, 0, len(r.GetChildren()))
		for _, c := range r.GetChildren() {
			if nc := rewriteExpr(c); nc != nil {
				body = append(body, nc)
			}
		}
		rules = append(rules, &gluonpb.ASTNode{
			Kind:     r.GetKind(),
			Value:    r.GetValue(),
			Children: body,
			Location: r.GetLocation(),
		})
	}
	return &gluonpb.ASTNode{Kind: root.GetKind(), Value: root.GetValue(), Children: rules}
}

// rewriteExpr rewrites one expression node; nil means "drop this node".
func rewriteExpr(n *gluonpb.ASTNode) *gluonpb.ASTNode {
	if n == nil {
		return nil
	}
	switch n.GetKind() {
	case compiler.KindTerminal:
		// The ":" separators are pure syntax; other terminals (none today
		// besides keys, which arrive as nonterminals) pass through.
		if n.GetValue() == ":" {
			return nil
		}
		return n
	case compiler.KindNonterminal:
		name := n.GetValue()
		if kw, ok := keywordRules[name]; ok {
			return &gluonpb.ASTNode{Kind: compiler.KindTerminal, Value: kw}
		}
		if scalarRules[name] {
			return &gluonpb.ASTNode{Kind: compiler.KindScalar, Value: name}
		}
		if droppedRules[name] {
			return nil
		}
		return n
	case compiler.KindSequence, compiler.KindAlternation,
		compiler.KindOptional, compiler.KindRepetition, compiler.KindGroup:
		kept := make([]*gluonpb.ASTNode, 0, len(n.GetChildren()))
		for _, c := range n.GetChildren() {
			if nc := rewriteExpr(c); nc != nil {
				kept = append(kept, nc)
			}
		}
		switch {
		case len(kept) == 0:
			return nil
		case len(kept) == 1 && n.GetKind() != compiler.KindOptional && n.GetKind() != compiler.KindRepetition:
			// A sequence/alternation/group of one is its child; optional and
			// repetition keep their wrapper semantics (presence, repeated).
			return kept[0]
		default:
			return &gluonpb.ASTNode{Kind: n.GetKind(), Value: n.GetValue(), Children: kept, Location: n.GetLocation()}
		}
	default:
		// range / range bounds / scalar — pass through untouched.
		return n
	}
}

// renderProtoFile prints one file from a resolved registry as .proto
// source. ForceFullyQualifiedNames mirrors kvq's genproto: bare relative
// names can re-resolve against nested wrapper messages and change meaning.
func renderProtoFile(files *protoregistry.Files, path string) (string, error) {
	fd, err := files.FindFileByPath(path)
	if err != nil {
		return "", fmt.Errorf("find %s: %w", path, err)
	}
	p := protoprint.Printer{ForceFullyQualifiedNames: true}
	var b strings.Builder
	if err := p.PrintProtoFile(fd, &b); err != nil {
		return "", fmt.Errorf("PrintProtoFile %s: %w", path, err)
	}
	return b.String(), nil
}
