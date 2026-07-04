# docs/ — shared context and learnings

Per the project README, `docs/` is the **active record of notes, shared context, and
learnings/findings** for proto-robotstxt. Its purpose is to consolidate authoritative,
grounded context *into this repo* so that future developers and agents can pull it
down without re-fetching or re-deriving it. If you learned something non-obvious while
working here (a spec quirk, a build gotcha, a divergence between parsers), it belongs
somewhere under `docs/`.

## Layout

| Path | What lives there |
|------|------------------|
| `docs/rfc/9309/` | RFC 9309 (Robots Exclusion Protocol). `raw.html`/`raw.txt` are **gitignored** (RFC Editor licensing) — rematerialize with `tools/rfc/pull-rfc.sh`. The checked-in `README.md` is our implementer-oriented summary with section citations. The extracted grammar itself lives at `grammar/rep.ebnf` (repo top level, not here). |
| `docs/google-dev-docs/` | Google Search Central / crawler docs pulled by `tools/google-dev/pull-docs.sh`. Extracted `<slug>.md` files (CC BY 4.0, with attribution headers) are committed; `rawhtml/` is gitignored. `README.md` there is the index — check its per-page extraction-quality notes before trusting an extracted file. |
| `docs/design/` | Design documents for cross-cutting decisions: [`two-tier-parsing.md`](design/two-tier-parsing.md) (the general strict+recovery pattern and its gluon support audit) and [`malformed-input.md`](design/malformed-input.md) (the robots.txt instance — phases, acceptance criteria, status). Keep status lines in these docs current as phases land. |
| `docs/progresslog/` | Per-task progress logs: one `<taskname>.md` per task, newest entries at top. See `docs/progresslog/README.md` for the convention. **Agents: actively document your progress there as you work.** |
| `docs/TODO.md` | The maintained project TODO list, ordered by priority. Add items as you discover work; move finished items to Done with a date and a progresslog pointer. |

## Rules of the road

- **Log your progress.** Every task gets a `docs/progresslog/<taskname>.md`; record
  decisions, gotchas, and current state as you go, not after.
- **Generated vs. authored.** Files produced by the pull scripts
  (`docs/google-dev-docs/<slug>.md`, `raw.*`) are generated artifacts — don't
  hand-edit them; fix the tool and re-run, or note problems in the relevant README.
- **Licensing.** Google dev-doc text is CC BY 4.0 (keep the attribution headers);
  RFC raw files stay gitignored.
- **Cite sources.** Summaries should cite RFC section numbers and source URLs so
  claims can be re-verified.
