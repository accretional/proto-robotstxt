package robotsgluon

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// TestRenderRoundTripCorpus pins guarantee 1 of render.go: for every
// strict-tier corpus file, parse → render → parse is the identity on the
// rep, and the rendered text deserializes to the same directive sequence
// (kinds/values; line numbers differ since comments and blank lines are
// not part of the rep).
func TestRenderRoundTripCorpus(t *testing.T) {
	g := mustGrammar(t)
	files, err := filepath.Glob("../testdata/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus: %v", err)
	}
	for _, f := range files {
		f := f
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			rep1, err := g.Rep(src)
			if err != nil {
				t.Fatal(err)
			}
			text, err := RenderRep(rep1, RenderOptions{Validate: true})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			rep2, err := g.Rep(text)
			if err != nil {
				t.Fatalf("rendered text failed strict parse: %v\n%s", err, text)
			}
			if !proto.Equal(rep1, rep2) {
				t.Errorf("rep round-trip not identity:\noriginal: %v\nreparsed: %v\nrendered text:\n%s", rep1, rep2, text)
			}
			// Directive sequence is preserved (modulo line numbers).
			ev1, err := g.Events(src)
			if err != nil {
				t.Fatal(err)
			}
			ev2, err := g.Events(text)
			if err != nil {
				t.Fatal(err)
			}
			if len(ev1) != len(ev2) {
				t.Fatalf("event count changed: %d -> %d", len(ev1), len(ev2))
			}
			for i := range ev1 {
				if ev1[i].Kind != ev2[i].Kind || ev1[i].Value != ev2[i].Value || ev1[i].Key != ev2[i].Key {
					t.Errorf("event %d changed: %s -> %s", i, ev1[i], ev2[i])
				}
			}
		})
	}
}

// mutateRep builds a Robotstxt rep with a deliberately invalid field.
func repWith(t *testing.T, set func(msg *dynamicpb.Message, md protoreflect.MessageDescriptor)) protoreflect.Message {
	t.Helper()
	md, err := repDescriptor("Robotstxt")
	if err != nil {
		t.Fatal(err)
	}
	msg := dynamicpb.NewMessage(md)
	set(msg, md)
	return msg
}

func TestRenderValidateRejects(t *testing.T) {
	g := mustGrammar(t)
	// Build a valid rep by parsing, then break one field via reflection.
	rep, err := g.Rep([]byte("User-agent: FooBot\nDisallow: /x\n"))
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt the product token (space is not an identifier char).
	item := rep.Get(rep.Descriptor().Fields().ByName("alt1")).List().Get(0).Message()
	grp := item.Get(item.Descriptor().Fields().ByName("group")).Message()
	sgl := grp.Get(grp.Descriptor().Fields().ByName("startgroupline")).Message()
	sgl.Set(sgl.Descriptor().Fields().ByName("product_token"), protoreflect.ValueOfString("Foo Bot"))

	if _, err := RenderRep(rep, RenderOptions{Validate: true}); err == nil {
		t.Error("validate should reject product_token with a space")
	}
	// Raw mode renders it anyway (adversarial text for fuzzing).
	out, err := RenderRep(rep, RenderOptions{})
	if err != nil {
		t.Fatalf("raw render: %v", err)
	}
	if string(out) != "User-agent: Foo Bot\nDisallow: /x\n" {
		t.Errorf("raw render = %q", out)
	}
}

func TestRenderEmptyRep(t *testing.T) {
	msg := repWith(t, func(m *dynamicpb.Message, md protoreflect.MessageDescriptor) {})
	out, err := RenderRep(msg, RenderOptions{Validate: true})
	if err != nil || len(out) != 0 {
		t.Fatalf("empty rep should render to empty text: %q, %v", out, err)
	}
}
