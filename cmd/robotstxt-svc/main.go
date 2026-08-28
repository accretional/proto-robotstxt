// Command robotstxt-svc serves proto-robotstxt over gRPC: it fetches a domain's
// robots.txt, parses it with the grammar-driven parser, and answers the two
// questions a crawler asks of it — what may I fetch, and where are the sitemaps.
//
//	robotstxt.svc.v1.RobotsService/Parse
//	robotstxt.svc.v1.RobotsService/Filter
//
// Server reflection is registered, so grpcurl needs no .proto to call it, and
// the standard grpc.health.v1.Health service answers health checks.
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
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	pb "github.com/accretional/proto-robotstxt/proto/pb"
	robots "github.com/accretional/proto-robotstxt/src-gluon"
)

// maxMessageBytes is generous because Filter carries a URL list: a 50,000-URL
// request is a few MiB, well past gRPC's 4 MiB default, and the failure mode
// would be an opaque ResourceExhausted at the transport layer.
const maxMessageBytes = 64 << 20

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
		log.Fatalf("robotstxt-svc: load grammar: %v", err)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("robotstxt-svc: listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageBytes),
		grpc.MaxSendMsgSize(maxMessageBytes),
	)
	pb.RegisterRobotsServiceServer(srv, &server{
		grammar: g,
		fetch:   newFetcher(*userAgent, *fetchTimeout),
	})

	// Health: the standard service, so Cloud Run and grpcurl both have a
	// well-known probe that is not an application endpoint.
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, hs)

	// Reflection: grpcurl can then call this service with no .proto on hand.
	reflection.Register(srv)

	// Drain in-flight RPCs on SIGTERM — Cloud Run sends one before shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Printf("robotstxt-svc: shutting down")
		hs.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		srv.GracefulStop()
	}()

	log.Printf("robotstxt-svc: serving gRPC on %s", *addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("robotstxt-svc: %v", err)
	}
}

type server struct {
	pb.UnimplementedRobotsServiceServer
	grammar *robots.Grammar
	fetch   *fetcher
}

func (s *server) Parse(ctx context.Context, req *pb.ParseRequest) (*pb.ParseResponse, error) {
	origin, err := originOf(req.GetDomain())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	agent := req.GetAgent()
	if agent == "" {
		agent = "*"
	}

	res := s.fetch.get(ctx, origin)
	resp := &pb.ParseResponse{
		Origin:     origin,
		RobotsUrl:  origin + "/robots.txt",
		Outcome:    res.Outcome,
		StatusCode: int32(res.StatusCode),
	}
	if res.Err != nil {
		resp.FetchError = res.Err.Error()
	}

	switch res.Outcome {
	case pb.FetchOutcome_FETCH_OUTCOME_UNAVAILABLE:
		resp.AllowAll = true
	case pb.FetchOutcome_FETCH_OUTCOME_UNREACHABLE:
		resp.DisallowAll = true
	case pb.FetchOutcome_FETCH_OUTCOME_SUCCESS:
		rec, err := s.grammar.Recover(res.Body)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "parse robots.txt: %v", err)
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
		if req.GetIncludeText() {
			resp.RobotsTxt = string(res.Body)
		}
	}

	log.Printf("Parse: %s outcome=%s status=%d tier=%s sitemaps=%d delay=%.3g",
		origin, resp.Outcome, resp.StatusCode, resp.Tier, len(resp.Sitemaps), resp.CrawlDelaySeconds)
	return resp, nil
}

func (s *server) Filter(ctx context.Context, req *pb.FilterRequest) (*pb.FilterResponse, error) {
	if req.GetAllowAll() && req.GetDisallowAll() {
		return nil, status.Error(codes.InvalidArgument, "allow_all and disallow_all are mutually exclusive")
	}
	agent := req.GetAgent()
	if agent == "" {
		agent = "*"
	}

	resp := &pb.FilterResponse{Allowed: []string{}, Disallowed: []string{}}

	switch {
	case req.GetDisallowAll():
		resp.Disallowed = append(resp.Disallowed, req.GetUrls()...)
	case req.GetAllowAll():
		resp.Allowed = append(resp.Allowed, req.GetUrls()...)
	default:
		rec, err := s.grammar.Recover([]byte(req.GetRobotsTxt()))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "parse robots.txt: %v", err)
		}
		agents := []string{agent}
		for _, u := range req.GetUrls() {
			if robots.AllowedByEvents(rec.Events, agents, u) {
				resp.Allowed = append(resp.Allowed, u)
			} else {
				resp.Disallowed = append(resp.Disallowed, u)
			}
		}
	}

	log.Printf("Filter: agent=%s urls=%d allowed=%d disallowed=%d",
		agent, len(req.GetUrls()), len(resp.Allowed), len(resp.Disallowed))
	return resp, nil
}
