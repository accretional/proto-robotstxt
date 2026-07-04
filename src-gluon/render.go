package robotsgluon

// render.go — the rep→text generator: renders a robotstxt.rep.Robotstxt
// message (proto/rep.proto, dynamicpb) back to robots.txt text. This is the
// "generating robots.txt files" tooling from the project README and the
// enabler for structure-aware fuzzing (mutate the rep, render, feed both
// parsers).
//
// Round-trip guarantees (tested):
//
//  1. For reps produced by PARSING (strict tier), render→parse is the
//     IDENTITY on the rep: parse(render(rep)) == rep. Formatting is
//     canonical ("User-agent: x", one space, "\n", blank line between
//     top-level items), which is fine — comments, blank lines and key
//     casing are not part of the rep.
//  2. For ARBITRARY reps (e.g. fuzz-mutated), rendering with Validate
//     DISABLED emits field bytes verbatim; the text may not reparse to the
//     same rep (a product_token with a space isn't a token anymore) — by
//     design: adversarial reps should produce adversarial text. With
//     Validate ENABLED, rendering errors out unless every field satisfies
//     its grammar rule, and identity holds.

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// RenderOptions configures RenderRep.
type RenderOptions struct {
	// Validate rejects reps whose fields don't satisfy their grammar rules
	// (product_token per RFC identifier|"*", path_pattern "/"-prefixed
	// UTF8-char-noctl, keys/values within their character classes), so the
	// output is guaranteed to reparse strictly to the same rep.
	Validate bool
}

// RenderRep renders a robotstxt.rep.Robotstxt message to robots.txt text.
func RenderRep(msg protoreflect.Message, opts RenderOptions) ([]byte, error) {
	if got, want := msg.Descriptor().Name(), protoreflect.Name("Robotstxt"); got != want {
		return nil, fmt.Errorf("RenderRep: got %s, want %s", got, want)
	}
	r := &renderer{validate: opts.Validate}
	items := msg.Get(msg.Descriptor().Fields().ByName("alt1")).List()
	for i := 0; i < items.Len(); i++ {
		if i > 0 {
			r.b.WriteString("\n")
		}
		if err := r.item(items.Get(i).Message()); err != nil {
			return nil, err
		}
	}
	return []byte(r.b.String()), r.err
}

type renderer struct {
	b        strings.Builder
	validate bool
	err      error
}

// item renders one Robotstxt.Alt1 / Group.Alt1 oneof arm.
func (r *renderer) item(m protoreflect.Message) error {
	d := m.Descriptor()
	which := m.WhichOneof(d.Oneofs().ByName("value"))
	if which == nil {
		return fmt.Errorf("render: empty oneof in %s", d.FullName())
	}
	inner := m.Get(which).Message()
	switch which.Name() {
	case "group":
		return r.group(inner)
	case "rule":
		return r.rule(inner)
	case "sitemapline":
		return r.line("Sitemap", getStr(inner, "any_value"), r.checkValue)
	case "otherline":
		key := getStr(inner, "other_key")
		if err := r.checkOtherKey(key); err != nil {
			return err
		}
		return r.line(key, getStr(inner, "any_value"), r.checkValue)
	default:
		return fmt.Errorf("render: unsupported item %s", which.FullName())
	}
}

func (r *renderer) group(m protoreflect.Message) error {
	d := m.Descriptor()
	if m.Has(d.Fields().ByName("startgroupline")) {
		if err := r.startgroupline(m.Get(d.Fields().ByName("startgroupline")).Message()); err != nil {
			return err
		}
	} else if r.validate {
		return fmt.Errorf("render: group without startgroupline")
	}
	extra := m.Get(d.Fields().ByName("startgroupline_2")).List()
	for i := 0; i < extra.Len(); i++ {
		if err := r.startgroupline(extra.Get(i).Message()); err != nil {
			return err
		}
	}
	items := m.Get(d.Fields().ByName("alt1")).List()
	for i := 0; i < items.Len(); i++ {
		if err := r.item(items.Get(i).Message()); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) startgroupline(m protoreflect.Message) error {
	token := getStr(m, "product_token")
	if r.validate {
		if !validProductToken(token) {
			return fmt.Errorf("render: invalid product_token %q", token)
		}
	}
	return r.line("User-agent", token, nil)
}

func (r *renderer) rule(m protoreflect.Message) error {
	key := "Allow"
	if m.Has(m.Descriptor().Fields().ByName("rule_key")) {
		ruleKey := m.Get(m.Descriptor().Fields().ByName("rule_key")).Message()
		if which := ruleKey.WhichOneof(ruleKey.Descriptor().Oneofs().ByName("value")); which != nil && which.Name() == "disallow_keyword" {
			key = "Disallow"
		}
	}
	path := getStr(m, "path_pattern")
	if r.validate && !validPathPattern(path) {
		return fmt.Errorf("render: invalid path_pattern %q", path)
	}
	return r.line(key, path, nil)
}

// line emits `key: value\n` (or `key:\n` for an empty value), validating
// the value when a checker is supplied and validation is on.
func (r *renderer) line(key, value string, check func(string) error) error {
	if r.validate && check != nil {
		if err := check(value); err != nil {
			return err
		}
	}
	r.b.WriteString(key)
	r.b.WriteString(":")
	if value != "" {
		r.b.WriteString(" ")
		r.b.WriteString(value)
	}
	r.b.WriteString("\n")
	return nil
}

func (r *renderer) checkValue(v string) error {
	if v == "" {
		return nil
	}
	if v != strings.Trim(v, " \t") {
		return fmt.Errorf("render: value %q has leading/trailing whitespace", v)
	}
	for i := 0; i < len(v); {
		if isWS(v[i]) {
			i++
			continue
		}
		n := utf8CharNoctlLen(v, i)
		if n == 0 {
			return fmt.Errorf("render: value %q has byte outside UTF8-char-noctl/WS at %d", v, i)
		}
		i += n
	}
	return nil
}

func (r *renderer) checkOtherKey(key string) error {
	if !r.validate {
		return nil // raw mode renders whatever the rep holds
	}
	if key == "" || strings.ContainsAny(key, ":#\r\n") || userAgentLike(key) ||
		key != strings.Trim(key, " \t") {
		return fmt.Errorf("render: invalid other_key %q", key)
	}
	return r.checkValue(key)
}

func validProductToken(t string) bool {
	if t == "*" {
		return true
	}
	if t == "" {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func validPathPattern(p string) bool {
	if p == "" {
		return true // empty-pattern rule ("Disallow:")
	}
	if p[0] != '/' {
		return false
	}
	for i := 1; i < len(p); {
		n := utf8CharNoctlLen(p, i)
		if n == 0 {
			return false
		}
		i += n
	}
	return true
}

func getStr(m protoreflect.Message, field string) string {
	return m.Get(m.Descriptor().Fields().ByName(protoreflect.Name(field))).String()
}
