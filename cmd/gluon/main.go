// Command gluon is the one-off CLI over the gluon-grammar robots.txt parser
// (src-gluon/): parse robots.txt files against the RFC 9309 EBNF
// formalization (grammar/rep.ebnf), lower parses to google's deserialized
// event form, cross-check against the vendored google parser, and derive the
// proto representation of the grammar. See src-gluon/README.md.
//
// Usage:
//
//	gluon [-grammar path] grammar                 validate + summarize the grammar
//	gluon [-grammar path] parse <file>            print the CST (textproto)
//	gluon [-grammar path] events <file>           print google-form parse events
//	gluon [-grammar path] check [-dump bin] <file>...
//	                                              cross-check gluon vs google events
//	gluon [-grammar path] genproto [-out dir]     grammar -> rep.{proto,fdset}
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"

	robotsgluon "github.com/accretional/proto-robotstxt/src-gluon"
)

func main() {
	grammarPath := flag.String("grammar", "", "path to a rep.ebnf-dialect grammar (default: embedded grammar/rep.ebnf)")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		usage()
		os.Exit(2)
	}

	g, err := loadGrammar(*grammarPath)
	if err != nil {
		fatal(err)
	}

	cmd, args := flag.Arg(0), flag.Args()[1:]
	switch cmd {
	case "grammar":
		err = cmdGrammar(*grammarPath)
	case "parse":
		err = cmdParse(g, args)
	case "events":
		err = cmdEvents(g, args)
	case "rep":
		err = cmdRep(g, args)
	case "meta":
		err = cmdMeta(args)
	case "check":
		err = cmdCheck(g, args)
	case "genproto":
		err = cmdGenproto(*grammarPath, args)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: gluon [-grammar path] <command> [args]

commands:
  grammar                  validate the grammar; print its rules
  parse <file>             parse a robots.txt; print the CST as textproto
  rep <file>               parse; print the typed rep (proto/rep.proto) as textproto
  events [-recover] <file> parse; print google-deserialization-form events
  meta <file>              print the per-line metadata stream (google
                           ReportLineMetadata form; pure line-local pass)
  check [-dump bin] [-recover] <f>...
                           parse each file with BOTH parsers; diff the events
  genproto [-out dir]      derive proto schema from the grammar -> rep.proto/.fdset
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gluon:", err)
	os.Exit(1)
}

func loadGrammar(path string) (*robotsgluon.Grammar, error) {
	if path == "" {
		return robotsgluon.Default()
	}
	return robotsgluon.LoadGrammar(path)
}

func cmdGrammar(path string) error {
	gd, err := robotsgluon.GrammarDescriptor(path)
	if err != nil {
		return err
	}
	fmt.Printf("grammar %s: %d rules (start rule: %s)\n", gd.GetName(), len(gd.GetRules()), gd.GetRules()[0].GetName())
	for _, r := range gd.GetRules() {
		kind := "grammar"
		if len(r.GetExpressions()) == 0 {
			kind = "token matcher"
		}
		fmt.Printf("  %-16s (%s)\n", r.GetName(), kind)
	}
	return nil
}

func cmdParse(g *robotsgluon.Grammar, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("parse: want exactly one file argument")
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	ast, err := g.Parse(src)
	if err != nil {
		return err
	}
	out, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(ast)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func cmdRep(g *robotsgluon.Grammar, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("rep: want exactly one file argument")
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	rep, err := g.Rep(src)
	if err != nil {
		return err
	}
	out, err := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(rep)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func cmdMeta(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("meta: want exactly one file argument")
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	for _, m := range robotsgluon.LineMetadataOf(src) {
		fmt.Println(m)
	}
	return nil
}

func cmdEvents(g *robotsgluon.Grammar, args []string) error {
	fs := flag.NewFlagSet("events", flag.ExitOnError)
	recover := fs.Bool("recover", false, "two-tier parse: fall back to line-level recovery when the strict parse fails (docs/design/malformed-input.md)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("events: want exactly one file argument")
	}
	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	var events []robotsgluon.Event
	if *recover {
		rec, err := g.Recover(src)
		if err != nil {
			return err
		}
		events = rec.Events
		fmt.Fprintln(os.Stderr, "gluon:", rec.RecoverSummary())
	} else {
		if events, err = g.Events(src); err != nil {
			return err
		}
	}
	for _, e := range events {
		fmt.Println(e)
	}
	return nil
}

func cmdCheck(g *robotsgluon.Grammar, args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	dump := fs.String("dump", defaultDumpBin(), "path to the robots_dump binary (tools/robots-dump)")
	recover := fs.Bool("recover", false, "two-tier parse: strict files must still cross-check, and files the strict grammar rejects are recovered line-by-line and cross-checked too")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()
	if len(files) == 0 {
		return fmt.Errorf("check: no input files")
	}
	if _, err := os.Stat(*dump); err != nil {
		return fmt.Errorf("check: robots_dump binary not found at %s (run ./build.sh or pass -dump)", *dump)
	}

	failures := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		var gluonEvents []robotsgluon.Event
		var gluonMeta []robotsgluon.LineMetadata
		tier := ""
		if *recover {
			rec, err := g.Recover(src)
			if err != nil {
				return fmt.Errorf("check %s: %w", f, err)
			}
			gluonEvents, gluonMeta = rec.Events, rec.Metadata
			if rec.Strict == nil {
				tier = " (recovered)"
			}
		} else {
			if gluonEvents, err = g.Events(src); err != nil {
				fmt.Printf("PARSE-ERROR %-40s %v\n", f, err)
				failures++
				continue
			}
		}
		google, err := robotsgluon.GoogleParse(*dump, f)
		if err != nil {
			return fmt.Errorf("check %s: %w", f, err)
		}
		diffs := robotsgluon.DiffEvents(gluonEvents, google.Events)
		if *recover {
			diffs = append(diffs, robotsgluon.DiffMetadata(gluonMeta, google.Metadata)...)
		}
		if len(diffs) > 0 {
			fmt.Printf("MISMATCH    %-40s %d record(s) differ%s\n", f, len(diffs), tier)
			for _, d := range diffs {
				fmt.Printf("            %s\n", d)
			}
			failures++
		} else {
			what := fmt.Sprintf("%d event(s)", len(gluonEvents))
			if *recover {
				what += fmt.Sprintf(" + %d metadata", len(gluonMeta))
			}
			fmt.Printf("PASS        %-40s %s agree%s\n", f, what, tier)
		}
	}
	if failures > 0 {
		return fmt.Errorf("check: %d of %d file(s) failed", failures, len(files))
	}
	return nil
}

// defaultDumpBin resolves gen/bin/robots_dump relative to the repo root by
// walking up from the working directory to the nearest go.mod, falling back
// to the plain relative path.
func defaultDumpBin() string {
	dir, err := os.Getwd()
	if err != nil {
		return "gen/bin/robots_dump"
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "gen", "bin", "robots_dump")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "gen/bin/robots_dump"
		}
		dir = parent
	}
}

func cmdGenproto(grammarPath string, args []string) error {
	fs := flag.NewFlagSet("genproto", flag.ExitOnError)
	out := fs.String("out", "gen", "output directory for rep.proto and rep.fdset")
	pkg := fs.String("package", "robotstxt.rep", "proto package name")
	goPkg := fs.String("go-package", "github.com/accretional/proto-robotstxt/gen/reppb;reppb", "go_package option")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	res, err := robotsgluon.Genproto(grammarPath, robotsgluon.GenprotoOptions{
		Package:   *pkg,
		GoPackage: *goPkg,
	})
	if err != nil {
		return err
	}
	protoOut := filepath.Join(*out, "rep.proto")
	fdsetOut := filepath.Join(*out, "rep.fdset")
	if err := os.WriteFile(protoOut, []byte(res.ProtoSrc), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(fdsetOut, res.FdsetBytes, 0o644); err != nil {
		return err
	}
	fmt.Printf("genproto: %d message(s) -> %s, %s\n", res.Messages, protoOut, fdsetOut)
	fmt.Println(strings.TrimRight(res.ProtoSrc, "\n"))
	return nil
}
