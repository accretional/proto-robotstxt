# robotstxt-svc — the HTTP/JSON service image

Newest entries at top.

## 2026-08-27 — deployed to Cloud Run; renamed robots-svc -> robotstxt-svc

`./deploy.sh` mirrors webrisk-svc's conventions (same project `speax-498608`,
`us-central1`, the `embedder` Artifact Registry repo, a per-service SA with no
project roles, `--no-allow-unauthenticated`, git-SHA image tags). It also runs
`gcloud auth configure-docker us-west1-docker.pkg.dev` itself — the first deploy
failed on an unauthenticated Artifact Registry push, and that is a host setup
step, not something a human should have to know.

Live at `https://robotstxt-svc-1041587693629.us-central1.run.app`. Sized small
(512Mi/1cpu, concurrency 80): robots.txt work is one small fetch and a parse.

Renamed from `robots-svc` throughout — binary, image, Cloud Run service, runtime
SA, and `cmd/` directory — so the deployed name matches the repo.

**Known quirk, not ours.** `GET /healthz` returns a Google HTML 404 that never
reaches the container (the container logs show only the POSTs). The same is true
of `webrisk`, a service this repo never touched, so it is environmental to this
project's network rather than something the service does — something ahead of
Cloud Run answers non-API paths. The API endpoints work correctly and
unauthenticated requests are still rejected with 403, so the pipeline is
unaffected; `/healthz` remains useful locally and under docker-compose. Worth
chasing if a Cloud Run health check is ever wanted.

## 2026-08-27 — cmd/robotstxt-svc landed, wired into run.sh

**Why.** proto-robotstxt is being containerized as one service in a crawl
pipeline (domain → webrisk → robots.txt → sitemap → URL list, orchestrated by a
separate harness repo). The parser takes bytes and the CLI takes files; the
pipeline needs something that takes a *domain*.

**What landed.**

- `cmd/robotstxt-svc/` — `POST /v1/robots:parse`, `POST /v1/robots:filter`,
  `GET /healthz`. Picked up by `build.sh` with no change, since it already does
  `go build -o gen/bin/ ./cmd/...`.
  - `api.go` — wire types as plain Go structs. **Not** protos, deliberately:
    `proto/rep.proto` and `proto/recover.proto` are generated from the grammar
    (CLAUDE.md rule 6), so a hand-written service message in `proto/` would blur
    what that directory means and risk being clobbered by the next `genproto`.
  - `fetch.go` — the RFC 9309 §2.3.1 status-code semantics.
  - `analyze.go` — sitemap collection and Crawl-delay extraction off the event
    stream.
- `Dockerfile.svc` — Go-only service image, 24.8 MB, distroless/static, tests
  run in the builder stage.
- `run.sh` step 6 — starts the built service against a local origin and asserts
  its `:filter` verdict matches `robots_main`'s exit code on the same triple.

**Why the service fetches its own robots.txt.** The status-code semantics are
robots-specific and counter-intuitive enough that they should live next to the
parser rather than in a generic orchestrator: **4xx (except 429) means allow
everything**, 429/5xx/DNS-failure means disallow everything, and a redirect chain
past 5 hops is a 404 (→ allow all), not a transport failure (→ disallow all).
Google explicitly warns against using 401/403 to mean "disallow" — they mean
*allow*. The table is already in this repo at `docs/rfc/9309/README.md` §8 and
`docs/google-dev-docs/crawling-docs-robots-txt-robots-txt-spec.md`; the
knowledgebase paid for itself here. All nine status classes are pinned by tests.

**The service always runs `-recover`.** Real-world robots.txt fails the strict
RFC grammar routinely — `www.nytimes.com` alone has 7 lines whose user-agent
tokens (`archive.org_bot`, `peer39_crawler/1.0`, `Poseidon Research Crawler`)
are outside RFC 9309's `product-token`. Strict tier rejects the whole document
at offset 4479; recovery returns all 25 of its sitemaps. The response reports
`tier` and `irregular_lines` so a caller can see which happened.

**Crawl-delay needed its own group walk.** Crawl-delay is not in RFC 9309 and
google's parser does not act on it, so it arrives as an `UNKNOWN` event with
`Key="Crawl-delay"` — there is no typed field to read. `crawlDelayOf` mirrors
`matcher.go`'s group tracking (a port of `RobotsMatcher::HandleUserAgent`):
consecutive user-agent lines merge, a user-agent line *after a rule* starts a new
group, and a group naming the agent beats the global `*` group. Sharing that
grouping rule is the point — the delay is then attributed to the same group whose
Allow/Disallow lines the matcher applies. Ten cases pin it, including the merge
and new-group boundaries.

**Image split.** `Dockerfile.svc` is not an extension of the root `Dockerfile`.
The root one is a CI image (bazelisk, abseil, googletest, the vendored C++
parser, `./run.sh` at build time). The C++ side is the differential-test *oracle*
— it belongs in CI, not in a deployment that never calls it. Shipping only the Go
binary takes the image to 24.8 MB with the grammar embedded, so the runtime needs
nothing from disk.

**Gate.** `./run.sh` green, including the new step 6. `go test ./cmd/robotstxt-svc/`
is 30 cases (status semantics, redirects, truncation, origin normalization,
sitemaps, crawl-delay grouping, both handlers).
