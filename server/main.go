package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/elfiyang16/sgrol-ma/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip on the server side
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
	errPb "google.golang.org/genproto/googleapis/rpc/errdetails"
)

const REAPEAT_COUNT = 1000

var (
	// grpc's own error code
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid token")
)

var (
	// port  = flag.Int("port", 50051, "the port to serve on")
	addrs = []string{":50051", ":50052"}

	sleep = flag.Duration("sleep", 0, "duration between changes in health")

	system = "" // empty system means all systems
)

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
	PermitWithoutStream: true,            // allow pings even when there are no active streams
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle:     30 * time.Second, //  default value is infinity.
	MaxConnectionAge:      30 * time.Second, // default value is infinity.
	MaxConnectionAgeGrace: 5 * time.Second,  // default value is infinity.
	Time:                  10 * time.Second, // time to wait to ping client, default to 2 hours
	Timeout:               1 * time.Second,  // time to wait for ping ack before close conn, default to 20 second
}

type ecServer struct {
	pb.UnimplementedEchoServer
	count map[string]int
	mu    sync.Mutex
	addr  string
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count[req.Message]++
	// track the number of time a message is repeated
	if s.count[req.Message] > REAPEAT_COUNT {
		st := status.New(codes.ResourceExhausted, "message is repeated")
		// append details to the status
		ds, err := st.WithDetails(
			&errPb.QuotaFailure{
				Violations: []*errPb.QuotaFailure_Violation{{
					Subject:     fmt.Sprintf("message: %s", req.GetMessage()),
					Description: "Limit on number of times a message can be repeated reached",
				}},
			},
		)
		if err != nil {
			return nil, st.Err()
		}
		return nil, ds.Err()
	}
	return &pb.EchoResponse{Message: req.Message}, nil
}

// Where this stream coming from?
// Server set up methods, and client calls on stub
func (s *ecServer) BidirectionalStreamingEcho(stream pb.Echo_BidirectionalStreamingEchoServer) error {
	for {
		in, err := stream.Recv() // Why it's a pointer to the request thought?
		if err != nil {
			fmt.Printf("server: error receiving from stream: %v\n", err)
			if err == io.EOF { // the stream is closed by the client
				return nil
			}
			return err
		}
		fmt.Printf("echoing message %q\n", in.Message)
		err = stream.Send(&pb.EchoResponse{Message: in.GetMessage()})
		if err != nil {
			fmt.Printf("server: error sending to stream: %v\n", err)
		}
	}
}

func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")
	return token == "some-secret-token-xxx" // ignore validation and just pretend to check against a string
}

func ensureValidToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	// Check the client's token to verify it's validity.
	if !valid(meta["authorization"]) {
		return nil, errInvalidToken
	}
	// If valid, let the request through.
	return handler(ctx, req)
}

func runHealthSvr(server *grpc.Server) {
	// Register health server
	healthcheck := health.NewServer() // include statusMap and updates (map[string]map[healthgrpc.Health_WatchServer]chan healthpb.HealthCheckResponse_ServingStatus))
	healthgrpc.RegisterHealthServer(server, healthcheck)

	// start healthserver in a subroutine
	go func() {
		next := healthpb.HealthCheckResponse_SERVING
		// Here manually set the health status of the server.
		for {
			healthcheck.SetServingStatus(system, next)
			// Simulate the changes in server health, take turns from unhealthy to healthy
			if next == healthpb.HealthCheckResponse_SERVING {
				next = healthpb.HealthCheckResponse_NOT_SERVING
			} else {
				next = healthpb.HealthCheckResponse_SERVING
			}
			time.Sleep(*sleep)
		}
	}()
}

// logger is to mock a sophisticated logging system. To simplify the example, we just print out the content.
func logger(format string, a ...interface{}) {
	fmt.Printf("LOG:\t"+format+"\n", a...)
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	if !valid(md["authorization"]) {
		return nil, errInvalidToken
	}
	m, err := handler(ctx, req)
	if err != nil {
		logger("error: %v", err)
	}
	return m, err
}

// wrappedStream wraps around the embedded grpc.ServerStream, and intercepts the RecvMsg and
// SendMsg method call.
type wrappedStream struct {
	grpc.ServerStream
}

// HOC, or Decorator on the ServerStream interface
func (w *wrappedStream) RecvMsg(m interface{}) error {
	logger("Received message: %v", m)
	return w.ServerStream.RecvMsg(m)
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	logger("Sending message: %v", m)
	return w.ServerStream.SendMsg(m)
}

func newWrappedStream(s grpc.ServerStream) grpc.ServerStream {
	return &wrappedStream{s}
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return errMissingMetadata
	}
	if !valid(md["authorization"]) {
		return errInvalidToken
	}
	// Interceptor has to call the handler itself
	err := handler(srv, newWrappedStream(ss))

	if err != nil {
		logger("error: %v", err)
	}
	return err
}

func startServer(addr string, opts []grpc.ServerOption) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(opts...)
	pb.RegisterEchoServer(s, &ecServer{
		addr:  addr,
		count: make(map[string]int),
	})
	runHealthSvr(s)
	log.Println("Server started at " + addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	flag.Parse()
	cert, err := tls.LoadX509KeyPair(data.Path("x509/server_cert.pem"), data.Path("x509/server_key.pem"))
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}

	opts := []grpc.ServerOption{
		// grpc.UnaryInterceptor(ensureValidToken),
		// Enable TLS for all incoming connections.
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
		grpc.UnaryInterceptor(unaryInterceptor),
		grpc.StreamInterceptor(streamInterceptor),
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	}
	s := grpc.NewServer(opts...)
	pb.RegisterEchoServer(s, &ecServer{
		count: make(map[string]int),
	})

	// runHealthSvr(s)

	// lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	// if err != nil {
	// 	log.Fatalf("failed to listen: %v", err)
	// }

	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	var wg sync.WaitGroup
	for _, addr := range addrs {
		wg.Add(1)
		// start the server in a goroutine
		go func(addr string, opts []grpc.ServerOption) {
			defer wg.Done()
			startServer(addr, opts)
		}(addr, opts)
	}
	wg.Wait()
}
