// Package grammar embeds the RFC 9309 EBNF formalization so Go binaries can
// load it without knowing the repo layout. rep.ebnf stays a top-level,
// human-first artifact; this package is just the embed shim (go:embed cannot
// reach outside a package directory).
package grammar

import _ "embed"

// RepEBNF is the contents of grammar/rep.ebnf.
//
//go:embed rep.ebnf
var RepEBNF []byte
