package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	// Interesting, move in & out, and have to have this weird aming for protoc to work
	data "github.com/elfiyang16/sgrol-ma/data"
	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
	"golang.org/x/oauth2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	// pb "github.com/elfiyang16/sgrol-ma/proto/echo"
)

var addr = flag.String("addr", "localhost:50051", "http service address")

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

	perRPC := oauth.NewOauthAccess(fetchToken())
	creds, err := credentials.NewClientTLSFromFile(data.Path("x509/ca_cert.pem"), "x.test.example.com")
	if err != nil {
		log.Fatalf("failed to load credentials: %v", err)
	}

	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(perRPC),
		grpc.WithTransportCredentials(creds),
	}

	conn, err := grpc.Dial(*addr, opts...)
	// Bypass the TLS check, but this is not a good idea :)
	// conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ecClient := pb.NewEchoClient(conn)

	// streamWithCancel(ecClient)

	// callUnaryEcho(ecClient, "hello world")

}
