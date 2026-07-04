package robotsgluon

// recoverproto.go — the typed proto for two-tier recovery results
// (docs/design/malformed-input.md phase 3). proto/rep.proto stays purely
// grammar-derived, so the recovery shapes live in a SIBLING file,
// recover.proto, in the same package/descriptor set. Irregular lines are
// not part of the grammar — their descriptor is hand-built here (it mirrors
// LineResult/LineMetadata, which mirror robots.cc) rather than derived.
//
// Byte-typed fields (text/key/value) are deliberate: recovered lines can
// carry arbitrary bytes (invalid UTF-8, control characters) that a proto3
// `string` would reject at marshal time.

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func strp(s string) *string { return &s }
func i32p(n int32) *int32   { return &n }

func field(name string, num int32, typ descriptorpb.FieldDescriptorProto_Type, opts ...func(*descriptorpb.FieldDescriptorProto)) *descriptorpb.FieldDescriptorProto {
	f := &descriptorpb.FieldDescriptorProto{
		Name:   strp(name),
		Number: i32p(num),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   typ.Enum(),
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func repeated(f *descriptorpb.FieldDescriptorProto) {
	f.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
}

func msgType(name string) func(*descriptorpb.FieldDescriptorProto) {
	return func(f *descriptorpb.FieldDescriptorProto) { f.TypeName = strp(name) }
}

// recoverFileDescriptor builds recover.proto: RecoveredRobotstxt wrapping
// the grammar-derived Robotstxt plus per-line records and google's
// per-line metadata (field order mirrors robots.h LineMetadata).
func recoverFileDescriptor(pkg string) *descriptorpb.FileDescriptorProto {
	const (
		tBool  = descriptorpb.FieldDescriptorProto_TYPE_BOOL
		tInt32 = descriptorpb.FieldDescriptorProto_TYPE_INT32
		tBytes = descriptorpb.FieldDescriptorProto_TYPE_BYTES
		tStr   = descriptorpb.FieldDescriptorProto_TYPE_STRING
		tMsg   = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	)
	fq := func(name string) string { return "." + pkg + "." + name }

	return &descriptorpb.FileDescriptorProto{
		Name:       strp("recover.proto"),
		Package:    strp(pkg),
		Syntax:     strp("proto3"),
		Dependency: []string{"rep.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: strp("RecoveredRobotstxt"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("strict", 1, tMsg, msgType(fq("Robotstxt"))),
					field("lines", 2, tMsg, msgType(fq("RecoveredLine")), repeated),
					field("metadata", 3, tMsg, msgType(fq("LineMetadata")), repeated),
				},
			},
			{
				Name: strp("RecoveredLine"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("line", 1, tInt32),
					field("text", 2, tBytes),
					field("rule", 3, tStr),
					field("irregular", 4, tBool),
					field("reason", 5, tStr),
					field("directive", 6, tMsg, msgType(fq("IrregularDirective"))),
				},
			},
			{
				Name: strp("IrregularDirective"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("key", 1, tBytes),
					field("value", 2, tBytes),
					field("kind", 3, tStr),
				},
			},
			{
				Name: strp("LineMetadata"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field("line", 1, tInt32),
					field("is_empty", 2, tBool),
					field("has_comment", 3, tBool),
					field("is_comment", 4, tBool),
					field("has_directive", 5, tBool),
					field("is_acceptable_typo", 6, tBool),
					field("is_line_too_long", 7, tBool),
					field("is_missing_colon_separator", 8, tBool),
				},
			},
		},
	}
}

// RecoveredToRep lowers a Recovered into a robotstxt.rep.RecoveredRobotstxt
// message. Strict-tier results embed the full grammar-derived Robotstxt;
// tier-2 results carry the per-line records; metadata is always present.
func RecoveredToRep(rec *Recovered) (*dynamicpb.Message, error) {
	md, err := repDescriptor("RecoveredRobotstxt")
	if err != nil {
		return nil, err
	}
	msg := dynamicpb.NewMessage(md)

	if rec.Strict != nil {
		strict, err := CSTToRep(rec.Strict)
		if err != nil {
			return nil, err
		}
		msg.Set(fieldOf(md, "strict"), protoreflect.ValueOfMessage(strict))
	}

	lineMD, err := repDescriptor("RecoveredLine")
	if err != nil {
		return nil, err
	}
	dirMD, err := repDescriptor("IrregularDirective")
	if err != nil {
		return nil, err
	}
	lines := msg.Mutable(fieldOf(md, "lines")).List()
	for _, l := range rec.Lines {
		lm := dynamicpb.NewMessage(lineMD)
		lm.Set(fieldOf(lineMD, "line"), protoreflect.ValueOfInt32(l.Line))
		lm.Set(fieldOf(lineMD, "text"), protoreflect.ValueOfBytes([]byte(l.Text)))
		if l.Rule != "" {
			lm.Set(fieldOf(lineMD, "rule"), protoreflect.ValueOfString(l.Rule))
		}
		if l.Irregular {
			lm.Set(fieldOf(lineMD, "irregular"), protoreflect.ValueOfBool(true))
			lm.Set(fieldOf(lineMD, "reason"), protoreflect.ValueOfString(l.Reason))
			if d := parseGoogleLine(l.Text); d.key != "" {
				dm := dynamicpb.NewMessage(dirMD)
				dm.Set(fieldOf(dirMD, "key"), protoreflect.ValueOfBytes([]byte(d.key)))
				dm.Set(fieldOf(dirMD, "value"), protoreflect.ValueOfBytes([]byte(d.value)))
				dm.Set(fieldOf(dirMD, "kind"), protoreflect.ValueOfString(string(d.kind)))
				lm.Set(fieldOf(lineMD, "directive"), protoreflect.ValueOfMessage(dm))
			}
		}
		lines.Append(protoreflect.ValueOfMessage(lm))
	}

	metaMD, err := repDescriptor("LineMetadata")
	if err != nil {
		return nil, err
	}
	metas := msg.Mutable(fieldOf(md, "metadata")).List()
	for _, m := range rec.Metadata {
		mm := dynamicpb.NewMessage(metaMD)
		mm.Set(fieldOf(metaMD, "line"), protoreflect.ValueOfInt32(m.Line))
		for _, fl := range []struct {
			name string
			on   bool
		}{
			{"is_empty", m.IsEmpty}, {"has_comment", m.HasComment},
			{"is_comment", m.IsComment}, {"has_directive", m.HasDirective},
			{"is_acceptable_typo", m.IsAcceptableTypo},
			{"is_line_too_long", m.IsLineTooLong},
			{"is_missing_colon_separator", m.IsMissingColonSeparator},
		} {
			if fl.on {
				mm.Set(fieldOf(metaMD, fl.name), protoreflect.ValueOfBool(true))
			}
		}
		metas.Append(protoreflect.ValueOfMessage(mm))
	}

	return msg, nil
}
