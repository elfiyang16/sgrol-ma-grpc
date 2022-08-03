package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	// Interesting, move in & out, and have to have this weird aming for protoc to work
	data "github.com/elfiyang16/sgrol-ma/data"
	ecpb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"
	"golang.org/x/oauth2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	// ecpb "github.com/elfiyang16/sgrol-ma/proto/echo"
)

var addr = flag.String("addr", "localhost:50051", "http service address")

func callUnaryEcho(client ecpb.EchoClient, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.UnaryEcho(ctx, &ecpb.EchoRequest{Message: message})
	if err != nil {
		log.Fatalf("client.UnaryEcho(_) = _, %v: ", err)
	}
	fmt.Println("UnaryEcho: ", resp.Message)
}

// fake simulation
func fetchToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: "some-secret-token-xxx",
	}
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
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	rgc := ecpb.NewEchoClient(conn)

	callUnaryEcho(rgc, "hello world")
}
