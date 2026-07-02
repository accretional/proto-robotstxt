# proto-robotstxt
Google's official robotstxt parser + EBNF Formalization per RFC 9309, and additional tools for generation and handling

## Overview

Vendor-in https://github.com/google/robotstxt to src-google/ and get it building + working with setup.sh, build.sh, test.sh, and run.sh (where build.sh calls setup.sh, test.sh calls build.sh, run.sh calls test.sh by default and CLAUDE.md instructs agents to ALWAYS make sure run.sh works e2e before git pushing).

Set up a github.com/accretional/gluon/v2 parser against the abnf spec from https://www.rfc-editor.org/rfc/rfc9309.html#name-authors-addresses within src-gluon/ and document it in src-gluon/README.md - get it fully working e2e in a way that allows it to be used as a one-off cli

Once the gluon parser is working, we'll have a representation of the robots.txt grammar and structure in .proto files. Then we'll implement a small "compiler" from these .proto files to the src-google deserialized format for robots.txt and test that both parsers produce the same deserialized data across several different inputs. Once that's working we'll set up fuzz/ and use something like https://github.com/google/libprotobuf-mutator with libfuzzer to fuzz our gluon parer.

Later we'lll add tooling for generating robots.txt files, modifying, etc. For now create a gen/ with the built binaries and protos gitignored, except for a single consolidated proto/rep.proto once ready. Create a cmd/gluon to demonstrate/test out the derived format.

We'll also create and maintain an active record of notes/shared context and learnings/findings in docs/ as we implement this project - agents, that means you (put it in CLAUDE.md too please). Actively document your progress in a task-specific md file in docs/progresslog/<taskname>.md. We'll keep a list of TODOs in docs/TODO.md. One first todo to set up is a reminder to eventually set up a parser or directly instruct LLMs to go over all the google docs from https://developers.google.com/search/docs/crawling-indexing and aggregate them into a knowledgebase in docs/google-dev-docs/ (try to extract the text; though, it's ok to save the raw html to docs/google-dev-docs/rawhtml/ - make sure to abide by their CC 4.0 terms regarding the text) with the tools to do so in tools/google-dev/pull-docs.sh). Create something similar for pulling the rc data (but leave that gitignored by default, bc of their licensing) into docs/rfc/9309/raw.html and summarize the important parts in the dir README.md. Extract the grammar into a top-level grammar/rep.ebnf The main purpose of these tasks is to consolidate context into this repo in a way that allows future developers to automatically pull down authoritative, grounded docs about this.

In general prefer to test aganst accretional.com/proto-robotstxt
