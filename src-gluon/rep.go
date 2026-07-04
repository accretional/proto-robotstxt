package robotsgluon

// rep.go — lowers a robots.txt CST into the typed proto representation
// (proto/rep.proto, package robotstxt.rep). No generated Go bindings are
// needed: the schema is re-derived from the embedded grammar at runtime
// (Genproto) and instantiated through dynamicpb, so the representation can
// never drift from the grammar. cmd/gluon's `rep` subcommand prints the
// resulting Robotstxt message as textproto.

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	gluonpb "github.com/accretional/gluon/v2/pb"
)

var repSchema struct {
	once  sync.Once
	types map[string]protoreflect.MessageDescriptor
	err   error
}

// repDescriptor returns the MessageDescriptor for a robotstxt.rep message,
// building the schema from the embedded grammar on first use.
func repDescriptor(name string) (protoreflect.MessageDescriptor, error) {
	repSchema.once.Do(func() {
		res, err := Genproto("", GenprotoOptions{Package: "robotstxt.rep"})
		if err != nil {
			repSchema.err = fmt.Errorf("derive rep schema: %w", err)
			return
		}
		var set descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(res.FdsetBytes, &set); err != nil {
			repSchema.err = fmt.Errorf("unmarshal rep fdset: %w", err)
			return
		}
		files, err := protodesc.NewFiles(&set)
		if err != nil {
			repSchema.err = fmt.Errorf("protodesc: %w", err)
			return
		}
		repSchema.types = map[string]protoreflect.MessageDescriptor{}
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			msgs := fd.Messages()
			for i := 0; i < msgs.Len(); i++ {
				m := msgs.Get(i)
				repSchema.types[string(m.Name())] = m
			}
			return true
		})
	})
	if repSchema.err != nil {
		return nil, repSchema.err
	}
	md, ok := repSchema.types[name]
	if !ok {
		return nil, fmt.Errorf("rep schema has no message %q", name)
	}
	return md, nil
}

// Rep parses src and lowers the CST to a robotstxt.rep.Robotstxt message.
func (g *Grammar) Rep(src []byte) (*dynamicpb.Message, error) {
	ast, err := g.Parse(src)
	if err != nil {
		return nil, err
	}
	return CSTToRep(ast)
}

// CSTToRep converts a parsed CST into the typed rep proto.
func CSTToRep(ast *gluonpb.ASTDescriptor) (*dynamicpb.Message, error) {
	md, err := repDescriptor("Robotstxt")
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	list := msg.Mutable(fieldOf(md, "alt1")).List()
	err = walkLines(ast.GetRoot(), func(n *gluonpb.ASTNode) error {
		switch n.GetKind() {
		case "group":
			item, err := newItem(md, "group")
			if err != nil {
				return err
			}
			grp, err := groupToRep(n)
			if err != nil {
				return err
			}
			item.Set(fieldOf(item.Descriptor(), "group"), protoreflect.ValueOfMessage(grp))
			list.Append(protoreflect.ValueOfMessage(item))
		case "sitemapline", "otherline":
			item, err := newItem(md, n.GetKind())
			if err != nil {
				return err
			}
			line, err := lineToRep(n)
			if err != nil {
				return err
			}
			item.Set(fieldOf(item.Descriptor(), n.GetKind()), protoreflect.ValueOfMessage(line))
			list.Append(protoreflect.ValueOfMessage(item))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// walkLines visits the top-level items (group/sitemapline/otherline) under
// robotstxt in source order, without descending into groups.
func walkLines(n *gluonpb.ASTNode, fn func(*gluonpb.ASTNode) error) error {
	if n == nil {
		return nil
	}
	switch n.GetKind() {
	case "group", "sitemapline", "otherline":
		return fn(n)
	}
	for _, c := range n.GetChildren() {
		if err := walkLines(c, fn); err != nil {
			return err
		}
	}
	return nil
}

// newItem builds a parent.Alt1 wrapper message (the oneof container the
// compiler derives for `{ a | b | c }`), given one of its arm names.
func newItem(parent protoreflect.MessageDescriptor, arm string) (*dynamicpb.Message, error) {
	nested := parent.Messages().ByName("Alt1")
	if nested == nil {
		return nil, fmt.Errorf("%s has no nested Alt1", parent.FullName())
	}
	m := dynamicpb.NewMessage(nested)
	if m.Descriptor().Fields().ByName(protoreflect.Name(arm)) == nil {
		return nil, fmt.Errorf("%s.Alt1 has no arm %q", parent.FullName(), arm)
	}
	return m, nil
}

func groupToRep(n *gluonpb.ASTNode) (*dynamicpb.Message, error) {
	md, err := repDescriptor("Group")
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)
	first := true
	extra := msg.Mutable(fieldOf(md, "startgroupline_2")).List()
	items := msg.Mutable(fieldOf(md, "alt1")).List()
	var walk func(*gluonpb.ASTNode) error
	walk = func(c *gluonpb.ASTNode) error {
		switch c.GetKind() {
		case "startgroupline":
			line, err := lineToRep(c)
			if err != nil {
				return err
			}
			if first {
				msg.Set(fieldOf(md, "startgroupline"), protoreflect.ValueOfMessage(line))
				first = false
			} else {
				extra.Append(protoreflect.ValueOfMessage(line))
			}
			return nil
		case "rule", "sitemapline", "otherline":
			item, err := newItem(md, c.GetKind())
			if err != nil {
				return err
			}
			line, err := lineToRep(c)
			if err != nil {
				return err
			}
			item.Set(fieldOf(item.Descriptor(), c.GetKind()), protoreflect.ValueOfMessage(line))
			items.Append(protoreflect.ValueOfMessage(item))
			return nil
		}
		for _, cc := range c.GetChildren() {
			if err := walk(cc); err != nil {
				return err
			}
		}
		return nil
	}
	for _, c := range n.GetChildren() {
		if err := walk(c); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

// lineToRep lowers a single directive-line node to its rep message.
func lineToRep(n *gluonpb.ASTNode) (*dynamicpb.Message, error) {
	switch n.GetKind() {
	case "startgroupline":
		md, err := repDescriptor("Startgroupline")
		if err != nil {
			return nil, err
		}
		m := dynamicpb.NewMessage(md)
		m.Set(fieldOf(md, "product_token"), protoreflect.ValueOfString(subtreeText(find(n, "product_token"))))
		return m, nil
	case "rule":
		md, err := repDescriptor("Rule")
		if err != nil {
			return nil, err
		}
		m := dynamicpb.NewMessage(md)
		keyMD, err := repDescriptor("RuleKey")
		if err != nil {
			return nil, err
		}
		key := dynamicpb.NewMessage(keyMD)
		arm := "allow_keyword"
		markerName := "AllowKeyword"
		if kindOfRuleKey(n) == Disallow {
			arm = "disallow_keyword"
			markerName = "DisallowKeyword"
		}
		markerMD, err := repDescriptor(markerName)
		if err != nil {
			return nil, err
		}
		key.Set(fieldOf(keyMD, arm), protoreflect.ValueOfMessage(dynamicpb.NewMessage(markerMD)))
		m.Set(fieldOf(md, "rule_key"), protoreflect.ValueOfMessage(key))
		if p := find(n, "path_pattern"); p != nil {
			m.Set(fieldOf(md, "path_pattern"), protoreflect.ValueOfString(subtreeText(p)))
		}
		return m, nil
	case "sitemapline":
		md, err := repDescriptor("Sitemapline")
		if err != nil {
			return nil, err
		}
		m := dynamicpb.NewMessage(md)
		m.Set(fieldOf(md, "any_value"), protoreflect.ValueOfString(subtreeText(find(n, "any_value"))))
		return m, nil
	case "otherline":
		md, err := repDescriptor("Otherline")
		if err != nil {
			return nil, err
		}
		m := dynamicpb.NewMessage(md)
		m.Set(fieldOf(md, "other_key"), protoreflect.ValueOfString(subtreeText(find(n, "other_key"))))
		m.Set(fieldOf(md, "any_value"), protoreflect.ValueOfString(subtreeText(find(n, "any_value"))))
		return m, nil
	}
	return nil, fmt.Errorf("lineToRep: unsupported node kind %q", n.GetKind())
}

func kindOfRuleKey(rule *gluonpb.ASTNode) EventKind {
	key := subtreeText(find(rule, "rule_key"))
	if hasFoldPrefix(key, "disallow") {
		return Disallow
	}
	return Allow
}

func fieldOf(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	return md.Fields().ByName(protoreflect.Name(name))
}

// NewRepMessage returns an empty message of the named robotstxt.rep type
// (e.g. "Robotstxt", "RecoveredRobotstxt") — the entry point for building
// or unmarshaling reps from outside the package (CLI render, fuzzers).
func NewRepMessage(name string) (*dynamicpb.Message, error) {
	md, err := repDescriptor(name)
	if err != nil {
		return nil, err
	}
	return dynamicpb.NewMessage(md), nil
}
