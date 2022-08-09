package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/elfiyang16/sgrol-ma/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip on the server side
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
	hwpb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/hello"
	errPb "google.golang.org/genproto/googleapis/rpc/errdetails"
)

type ecServer struct {
	pb.UnimplementedEchoServer
	count map[string]int
	mu    sync.Mutex
	addr  string

	reqCounter uint
	reqModulo  uint
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Create trailer in defer to record function return time.
	defer func() {
		trailer := metadata.Pairs(
			"timestamp", time.Now().Format(timestampFormat),
		)
		grpc.SetTrailer(ctx, trailer)
	}()
	// read the metadata from the incoming context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.DataLoss, "failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Printf("timestamp from metadata:\n")
		for i, e := range t { // md value is []string
			fmt.Printf(" %d. %s\n", i, e)
		}
	}
	// create and send header
	header := metadata.New(map[string]string{
		"location":  "MTV",
		"timestamp": time.Now().Format(timestampFormat),
	})
	if err := grpc.SendHeader(ctx, header); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to send header: %v", err)
	}

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
	log.Printf("UnaryEcho server: %s", req.Message)

	// if err := s.maybeFailRequest(); err != nil {
	// 	log.Println("request failed count:", s.reqCounter)
	// 	return nil, err
	// }

	log.Println("request succeeded count:", s.reqCounter)
	return &pb.EchoResponse{Message: req.Message}, nil
}

// Echo_ServerStreamingEchoServer can only send
func (s *ecServer) ServerStreamingEcho(in *pb.EchoRequest, stream pb.Echo_ServerStreamingEchoServer) error {
	fmt.Printf("--- ServerStreamingEcho ---\n")
	defer func() {
		trailer := metadata.Pairs(
			"timestamp", time.Now().Format(timestampFormat),
		)
		stream.SetTrailer(trailer)
	}()

	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.DataLoss, "ServerStreamingEcho: failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Printf("timestamp from metadata:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	}
	// Create and send header.
	header := metadata.New(map[string]string{"location": "MTV", "timestamp": time.Now().Format(timestampFormat)})
	stream.SendHeader(header)

	fmt.Printf("request received: %v\n", in)

	for i := 0; i < streamingCount; i++ {
		if err := stream.Send(&pb.EchoResponse{Message: in.Message}); err != nil {
			return err
		}

	}
	return nil // denote finishing the response --> RPC translates to appropriate status code
}

// Echo_ClientStreamingEchoServer can  sendandclose, recv
func (s *ecServer) ClientStreamingEcho(stream pb.Echo_ClientStreamingEchoServer) error {
	fmt.Printf("--- ClientStreamingEcho ---\n")
	defer func() {
		trailer := metadata.Pairs(
			"timestamp", time.Now().Format(timestampFormat),
		)
		stream.SetTrailer(trailer)
	}()

	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.DataLoss, "ClientStreamingEcho: failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Printf("timestamp from metadata:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	}
	// Create and send header.
	header := metadata.New(map[string]string{"location": "MTV", "timestamp": time.Now().Format(timestampFormat)})
	if err := stream.SendHeader(header); err != nil {
		return err
	}

	var message string
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			fmt.Printf("echo last received message\n")
			// receive the last msg from client and send the response
			return stream.SendAndClose(&pb.EchoResponse{Message: message})
		}
		message = in.Message
		fmt.Printf("request received: %v, building echo\n", in)
		if err != nil {
			return err
		}
	}
}

// Where this stream coming from?
// Server set up methods, and client calls on stub
func (s *ecServer) BidirectionalStreamingEcho(stream pb.Echo_BidirectionalStreamingEchoServer) error {
	fmt.Printf("--- BidirectionalStreamingEcho ---\n")
	defer func() {
		trailer := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
		stream.SetTrailer(trailer)
	}()
	// Read metadata from client.
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.DataLoss, "BidirectionalStreamingEcho: failed to get metadata")
	}
	if t, ok := md["timestamp"]; ok {
		fmt.Printf("timestamp from metadata:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	}

	// Create and send header.
	header := metadata.New(map[string]string{"location": "MTV", "timestamp": time.Now().Format(timestampFormat)})
	stream.SendHeader(header)

	for {
		in, err := stream.Recv() // Why it's a pointer to the request thought?
		if err != nil {
			fmt.Printf("server: error receiving from stream: %v\n", err)
			// the stream is closed by the client, so server also closes
			if err == io.EOF {
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

// this method will fail reqModulo - 1 times RPCs and return status code Unavailable,
// and succeeded RPC on reqModulo times.
func (s *ecServer) maybeFailRequest() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqCounter++ // avoid race con on the atomic counter
	if (s.reqModulo > 0) && (s.reqCounter%s.reqModulo == 0) {
		return nil
	}
	log.Println("return unavailable res count", s.reqCounter)
	return status.Errorf(codes.Unavailable, "maybeFailRequest: failing it")
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
		// Configure server to pass every fourth RPC;
		// client is configured to make four attempts.
		// reqCounter: 0,
		// reqModulo:  4,
	})
	hwpb.RegisterGreeterServer(s, &hwServer{})
	runHealthSvr(s)
	runRelectionSvr(s)
	runChannelzSvr()
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
	// s := grpc.NewServer(opts...)
	// pb.RegisterEchoServer(s, &ecServer{
	// 	count: make(map[string]int),
	// })
	// register another service as multiplex
	// hwpb.RegisterGreeterServer(s, &hwServer{})

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
