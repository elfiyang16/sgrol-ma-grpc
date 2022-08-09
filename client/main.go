package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	// Interesting, move in & out, and have to have this weird aming for protoc to work

	pb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/echo"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	// pb "github.com/elfiyang16/sgrol-ma/proto/echo"

	// import grpc/health to enable transparent client side checking
	_ "google.golang.org/grpc/health"
)

func callUnaryEcho(client pb.EchoClient, message string) {
	fmt.Printf("--- unary ---\n")
	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx, cancel := context.WithTimeout(context.Background(), 39*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, md)
	defer cancel()

	var header, trailer metadata.MD

	resp, err := client.UnaryEcho(
		ctx,
		&pb.EchoRequest{Message: message},
		grpc.Header(&header), // Similar to json
		grpc.Trailer(&trailer),
		grpc.WaitForReady(true), // default is false
	)
	if err != nil {
		log.Fatalf("client.UnaryEcho(_) = _, %v: ", err)
	}

	checkHeader(header)
	checkTrailer(trailer)
	fmt.Println("UnaryEcho Res: ", resp.Message)
}

func callServerStreamingEcho(client pb.EchoClient, message string) {
	fmt.Printf("--- server streaming ---\n")
	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, md)
	defer cancel()

	stream, err := client.ServerStreamingEcho(
		ctx,
		&pb.EchoRequest{Message: message},
	)
	if err != nil {
		log.Fatalf("failed to call ServerStreamingEcho: %v", err)
	}
	// check header before receive
	header, err := stream.Header()
	if err != nil {
		log.Fatalf("failed to get header from stream: %v", err)
	}
	checkHeader(header)

	// receive messages from server
	var rpcStatus error
	for {
		res, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break // break on first err
		}
		fmt.Println("ServerStreamingEcho Res: ", res.Message)
	}
	if rpcStatus != io.EOF { // server side: return nil
		log.Fatalf("failed to finish server streaming: %v", rpcStatus)
	}

	// check trailer after receive
	trailer := stream.Trailer()
	checkTrailer(trailer)
}

func callClientStreamingEcho(client pb.EchoClient, message string) {
	fmt.Printf("--- client streaming ---\n")
	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, md)
	defer cancel()

	stream, err := client.ClientStreamingEcho(ctx)
	if err != nil {
		log.Fatalf("failed to call ClientStreamingEcho: %v", err)
	}

	// check header before send
	header, err := stream.Header()
	if err != nil {
		log.Fatalf("failed to get header from stream: %v", err)
	}
	checkHeader(header)

	// send stream
	for i := 0; i < streamingCount; i++ {
		if err := stream.Send(&pb.EchoRequest{Message: message}); err != nil {
			log.Fatalf("failed to send message streaming client: %v", err)
		}
	}

	res, err := stream.CloseAndRecv() // server side: SendAndClose
	if err != nil {
		log.Fatalf("failed to CloseAndRecv: %v\n", err)
	}
	fmt.Printf("response:\n")
	fmt.Printf(" - %s\n\n", res.Message)

	// check trailer after send
	trailer := stream.Trailer()
	checkTrailer(trailer)
}

func callBidiStreamingEcho(client pb.EchoClient, message string) {
	fmt.Printf("--- bidirectional ---\n")

	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, md)
	defer cancel()

	stream, err := client.BidirectionalStreamingEcho(ctx)
	if err != nil {
		return
	}

	// send on a GRT
	go func() {
		header, err := stream.Header()
		if err != nil {
			log.Fatalf("failed to get header from stream: %v", err)
		}
		checkHeader(header)

		for i := 0; i < 5; i++ {
			if err := stream.Send(&pb.EchoRequest{Message: message}); err != nil {
				log.Fatalf("failed to send message: %v", err)
			}
		}
		err = stream.CloseSend() // close the stream on the sender side
		if err != nil {
			log.Fatalf("failed to close send: %v", err)
		}
	}()

	// receive on current GRT
	var rpcStatus error
	for {
		resp, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break
		}
		fmt.Println("BidiStreamingEcho: ", resp.Message)
	}
	if rpcStatus != io.EOF {
		log.Fatalf("failed to finish server streaming: %v", rpcStatus)
	}
	trailer := stream.Trailer()
	checkTrailer(trailer)
}

func makeRPCs(cc *grpc.ClientConn, n int) {
	hwc := pb.NewEchoClient(cc)
	for i := 0; i < n; i++ {
		callUnaryEcho(hwc, "this is examples/load_balancing")
	}
}

func main() {
	flag.Parse()

	runChannelzSvr()

	conn := dialNormal()
	// conn := dialRoundRobin()
	// conn := dialPickFirst()
	defer conn.Close()

	ecClient := pb.NewEchoClient(conn)
	callUnaryEcho(ecClient, "Try and Success")
	callBidiStreamingEcho(ecClient, "hello world bidirection")
	callServerStreamingEcho(ecClient, "hello world server")
	callClientStreamingEcho(ecClient, "hello world client")

	fmt.Println("--- calling helloworld.Greeter/SayHello ---")
	// hwc := hwpb.NewGreeterClient(conn)
	// callSayHello(hwc, "multiplex")

	// callUnaryWithHealthConfig(ecClient)

	// streamWithCancel(ecClient)

	// callUnaryEcho(ecClient, "hello world")

	// callUnaryWithGzip(ecClient)

	// callUnaryEchoWithErrorQuota(ecClient)
}
