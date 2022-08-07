package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	// Interesting, move in & out, and have to have this weird aming for protoc to work
	data "github.com/elfiyang16/sgrol-ma/data"
	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
	"golang.org/x/oauth2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"

	// pb "github.com/elfiyang16/sgrol-ma/proto/echo"
	errPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	// import grpc/health to enable transparent client side checking
	_ "google.golang.org/grpc/health"
)

var addr = flag.String("addr", "localhost:50051", "http service address")

// valid json
var servinceConfig = `{
	"loadBalancingPolicy": "round_robin",
	"healthCheckConfig": {
		"serviceName": ""
	}
}`

func callUnaryEcho(client pb.EchoClient, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.UnaryEcho(ctx, &pb.EchoRequest{Message: message})
	if err != nil {
		log.Fatalf("client.UnaryEcho(_) = _, %v: ", err)
	}
	fmt.Println("UnaryEcho: ", resp.Message)
}

func streamWithCancel(client pb.EchoClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Init the stream with cancellable context
	stream, err := client.BidirectionalStreamingEcho(ctx)
	if err != nil {
		log.Fatalf("error creating stream: %v", err)
	}

	// Send and receive must match (what if no listening?)
	if err := sendMessage(stream, "hello"); err != nil {
		log.Fatalf("error sending message: %v", err)
	}
	if err := sendMessage(stream, "world"); err != nil {
		log.Fatalf("error sending on stream: %v", err)
	}
	recvMessage(stream, codes.OK)
	recvMessage(stream, codes.OK)

	fmt.Println("cancelling context")
	cancel()
	// This Send may or may not return an error, depending on whether the
	// monitored context detects cancellation before the call is made.
	sendMessage(stream, "closed")
	// This Recv should never succeed.
	// recvMessage(stream, codes.OK)

	recvMessage(stream, codes.Canceled)
}

func callUnaryWithGzip(client pb.EchoClient) {
	const msg = "compress"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.UnaryEcho(ctx, &pb.EchoRequest{Message: msg}, grpc.UseCompressor(gzip.Name))
	fmt.Printf("UnaryEcho: %v, %v\n", res.GetMessage(), err)
	if err != nil || res.GetMessage() != msg {
		log.Fatalf("Message=%q, err=%v; want Message=%q, err=<nil>", res.GetMessage(), err, msg)
	}
}

func callUnaryEchoWithErrorQuota(client pb.EchoClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := client.UnaryEcho(ctx, &pb.EchoRequest{Message: "hello"})
	if err != nil {
		s := status.Convert(err)
		for _, d := range s.Details() {
			switch info := d.(type) {
			case *errPb.QuotaFailure:
				log.Printf("QuotaInfo: %s", info)
			default:
				log.Printf("Unexpected type: %s", info)
			}
		}
		os.Exit(1)
	}
	log.Printf("Echo from server: %s", r.Message)
}

func callUnaryWithHealthConfig(client pb.EchoClient) {
	for {
		callUnaryEcho(client, "hello")
		time.Sleep(time.Second)
	}
}

// Authentication - fake simulation
func fetchToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: "some-secret-token-xxx",
	}
}

func sendMessage(stream pb.Echo_BidirectionalStreamingEchoClient, msg string) error {
	fmt.Printf("sending message %q\n", msg)
	return stream.Send(&pb.EchoRequest{Message: msg})
}

func recvMessage(stream pb.Echo_BidirectionalStreamingEchoClient, wantErrCode codes.Code) {
	res, err := stream.Recv() //response
	if status.Code(err) != wantErrCode {
		log.Fatalf("stream.Recv() = %v, %v; want _, status.Code(err)=%v", res, err, wantErrCode)
	}
	if err != nil {
		fmt.Printf("stream.Recv() returned expected error %v\n", err)
		return
	}
	fmt.Printf("Received message: %q\n", res.GetMessage())
}

func main() {
	flag.Parse()

	r := manual.NewBuilderWithScheme("whatever")
	r.InitialState(resolver.State{
		Addresses: []resolver.Address{
			{Addr: "localhost:50051"},
			{Addr: "localhost:50052"},
		},
	})
	address := fmt.Sprintf("%s:///unused", r.Scheme())

	perRPC := oauth.NewOauthAccess(fetchToken())
	creds, err := credentials.NewClientTLSFromFile(data.Path("x509/ca_cert.pem"), "x.test.example.com")
	if err != nil {
		log.Fatalf("failed to load credentials: %v", err)
	}

	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(perRPC),
		// Bypass the TLS check, but this is not a good idea :)
		// grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(creds),
		grpc.WithBlock(),
		grpc.WithResolvers(r), // block until underlying connection is up
		grpc.WithDefaultServiceConfig(servinceConfig),
	}

	// conn, err := grpc.Dial(*addr, opts...)
	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ecClient := pb.NewEchoClient(conn)

	callUnaryWithHealthConfig(ecClient)

	// streamWithCancel(ecClient)

	// callUnaryEcho(ecClient, "hello world")

	// callUnaryWithGzip(ecClient)

	// callUnaryEchoWithErrorQuota(ecClient)

}
