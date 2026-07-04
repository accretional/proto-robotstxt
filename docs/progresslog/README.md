# progresslog/ — per-task progress logs

Convention (from the project README: "Actively document your progress in a
task-specific md file in docs/progresslog/<taskname>.md"):

- **One file per task**, named `<taskname>.md` (short, stable slug, e.g.
  `bootstrap.md`, `docs.md`, `gluon-parser.md`). If you pick up an existing task,
  append to its existing file rather than creating a new one.
- **Newest entries at the top.** Start each entry with a `## YYYY-MM-DD — <who/what>`
  heading so the current state is the first thing a reader sees.
- **Record, minimally:**
  - **Decisions** made and *why* (especially where alternatives were considered);
  - **Gotchas** — anything that cost you time or would surprise the next agent
    (spec quirks, tool bugs, environment issues);
  - **State** — what works now, what is unfinished, and exact next steps, so someone
    else (or future you) can resume without re-discovery.
- Log **as you work**, not retroactively at the end. Link to files/URLs/sections
  rather than restating them.
- Durable, task-independent learnings should *also* be promoted to the relevant
  place in `docs/` (e.g. an RFC quirk goes in `docs/rfc/9309/README.md`), and new
  work items go to `docs/TODO.md` — the progress log is the narrative record, not
  the only home of the information.
