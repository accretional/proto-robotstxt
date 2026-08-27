// Command robots-svc serves proto-robotstxt over HTTP/JSON: it fetches a
// domain's robots.txt, parses it with the grammar-driven parser, and answers the
// two questions a crawler asks of it — what may I fetch, and where are the
// sitemaps.
//
//	POST /v1/robots:parse    {"domain":"example.com"} -> sitemaps, crawl delay, verdict
//	POST /v1/robots:filter   {"robots_txt":..., "urls":[...]} -> allowed / disallowed
//	GET  /healthz
//
// The service ships the Go parser only. The vendored C++ parser is the
// differential-test oracle (run.sh, `gluon check`) and has no place in a
// deployment image; see Dockerfile.svc.
//
// Parsing always runs the two-tier path (src-gluon Recover), because real-world
// robots.txt frequently fails the strict RFC 9309 grammar — non-conforming
// user-agent tokens alone are enough — and a crawler still has to act on it.
// The response says which tier answered.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	robots "github.com/accretional/proto-robotstxt/src-gluon"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address (PORT overrides the port)")
	fetchTimeout := flag.Duration("fetch-timeout", 20*time.Second, "robots.txt fetch timeout")
	userAgent := flag.String("user-agent", DefaultUserAgent, "User-Agent sent when fetching robots.txt")
	flag.Parse()

	if port := os.Getenv("PORT"); port != "" {
		*addr = ":" + port
	}

	// Load the grammar once: it is the expensive startup step, and *Grammar is
	// safe for concurrent use (src-gluon TestConcurrentGrammarUse).
	g, err := robots.Default()
	if err != nil {
		log.Fatalf("robots-svc: load grammar: %v", err)
	}

	s := &server{grammar: g, fetch: newFetcher(*userAgent, *fetchTimeout)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/robots:parse", s.parse)
	mux.HandleFunc("POST /v1/robots:filter", s.filter)
	mux.HandleFunc("GET /healthz", s.healthz)

	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("robots-svc: listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("robots-svc: %v", err)
	}
}

type server struct {
	grammar *robots.Grammar
	fetch   *fetcher
}

func (s *server) parse(w http.ResponseWriter, r *http.Request) {
	var req ParseRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	origin, err := originOf(req.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agent := req.Agent
	if agent == "" {
		agent = "*"
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.fetch.client.Timeout+5*time.Second)
	defer cancel()

	res := s.fetch.get(ctx, origin)
	resp := ParseResponse{
		Origin:     origin,
		RobotsURL:  origin + "/robots.txt",
		Outcome:    res.Outcome,
		StatusCode: res.StatusCode,
		Sitemaps:   []string{},
	}
	if res.Err != nil {
		resp.FetchError = res.Err.Error()
	}

	switch res.Outcome {
	case OutcomeUnavailable:
		resp.AllowAll = true
	case OutcomeUnreachable:
		resp.DisallowAll = true
	case OutcomeSuccess:
		rec, err := s.grammar.Recover(res.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "parse robots.txt: "+err.Error())
			return
		}
		resp.Tier = "recovered"
		if rec.Strict != nil {
			resp.Tier = "strict"
		}
		for _, l := range rec.Lines {
			if l.Irregular {
				resp.IrregularLines++
			}
		}
		resp.Sitemaps = sitemapsOf(rec.Events, origin)
		resp.CrawlDelaySeconds = crawlDelayOf(rec.Events, agent)
		if req.IncludeText {
			resp.RobotsTxt = string(res.Body)
		}
	}

	log.Printf("parse: %s outcome=%s status=%d tier=%s sitemaps=%d delay=%.3g",
		origin, resp.Outcome, resp.StatusCode, resp.Tier, len(resp.Sitemaps), resp.CrawlDelaySeconds)
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) filter(w http.ResponseWriter, r *http.Request) {
	var req FilterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.AllowAll && req.DisallowAll {
		writeError(w, http.StatusBadRequest, "allow_all and disallow_all are mutually exclusive")
		return
	}
	agent := req.Agent
	if agent == "" {
		agent = "*"
	}

	resp := FilterResponse{Allowed: []string{}, Disallowed: []string{}}

	switch {
	case req.DisallowAll:
		resp.Disallowed = append(resp.Disallowed, req.URLs...)
	case req.AllowAll:
		resp.Allowed = append(resp.Allowed, req.URLs...)
	default:
		rec, err := s.grammar.Recover([]byte(req.RobotsTxt))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "parse robots.txt: "+err.Error())
			return
		}
		agents := []string{agent}
		for _, u := range req.URLs {
			if robots.AllowedByEvents(rec.Events, agents, u) {
				resp.Allowed = append(resp.Allowed, u)
			} else {
				resp.Disallowed = append(resp.Disallowed, u)
			}
		}
	}

	log.Printf("filter: agent=%s urls=%d allowed=%d disallowed=%d",
		agent, len(req.URLs), len(resp.Allowed), len(resp.Disallowed))
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
