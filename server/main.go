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

	"github.com/elfiyang16/sgrol-ma/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/encoding/gzip" // Enable gzip on the server side
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
)

var (
	// grpc's own error code
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid token")
)

var port = flag.Int("port", 50051, "the port to serve on")

type ecServer struct {
	pb.UnimplementedEchoServer
}

func (s *ecServer) UnaryEcho(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
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

func main() {
	flag.Parse()
	fmt.Printf("server starting on port %d...\n", *port)

	cert, err := tls.LoadX509KeyPair(data.Path("x509/server_cert.pem"), data.Path("x509/server_key.pem"))
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(ensureValidToken),
		// Enable TLS for all incoming connections.
		grpc.Creds(credentials.NewServerTLSFromCert(&cert)),
	}
	s := grpc.NewServer(opts...)

	// Register EchoService service.
	pb.RegisterEchoServer(s, &ecServer{})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
