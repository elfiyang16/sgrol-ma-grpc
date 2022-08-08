package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/status"

	// pb "github.com/elfiyang16/sgrol-ma/proto/echo"
	errPb "google.golang.org/genproto/googleapis/rpc/errdetails"
	// import grpc/health to enable transparent client side checking
	_ "google.golang.org/grpc/health"
)

var addr = flag.String("addr", "localhost:50051", "http service address")

const (
	fallbackToken   = "some-secret-token"
	timestampFormat = time.StampNano // "Jan _2 15:04:05.000"
	streamingCount  = 10
)

// valid json
var servinceConfig = `{
	"loadBalancingPolicy": "round_robin",
	"healthCheckConfig": {
		"serviceName": ""
	}
}`

var kacp = keepalive.ClientParameters{
	Time:                10 * time.Second,
	Timeout:             20 * time.Second,
	PermitWithoutStream: true,
}

func logger(format string, a ...interface{}) {
	fmt.Printf("LOG:\t"+format+"\n", a...)
}

func unaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var credsConfigured bool
	for _, o := range opts {
		_, ok := o.(grpc.PerRPCCredsCallOption)
		if ok {
			credsConfigured = true
			break
		}
	}

	if !credsConfigured {
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(&oauth2.Token{AccessToken: fallbackToken})))
	}
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	end := time.Now()
	logger("RPC: %s, start time: %s, end time: %s, err: %v", method, start.Format("Basic"), end.Format(time.RFC3339), err)
	return err
}

type wrappedStream struct {
	grpc.ClientStream
}

func (w *wrappedStream) SendMsg(m interface{}) error {
	logger("Send a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))
	return w.ClientStream.SendMsg(m)
}

func (w *wrappedStream) RecvMsg(m interface{}) error {
	logger("Send a message (Type: %T) at %v", m, time.Now().Format(time.RFC3339))
	return w.ClientStream.RecvMsg(m)
}

func newWrappedStream(stream grpc.ClientStream) *wrappedStream {
	return &wrappedStream{stream}
}

// Streamer is called by StreamClientInterceptor to create a ClientStream.
func streamInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	var credsConfigured bool
	for _, o := range opts {
		_, ok := o.(*grpc.PerRPCCredsCallOption)
		if ok {
			credsConfigured = true
			break
		}
	}
	if !credsConfigured {
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(&oauth2.Token{
			AccessToken: fallbackToken,
		})))
	}
	clientStream, err := streamer(ctx, desc, cc, method, opts...)
	if err != nil {
		return nil, err
	}
	return newWrappedStream(clientStream), nil
}

func checkHeader(header metadata.MD) {
	if t, ok := header["timestamp"]; ok {
		fmt.Printf("timestamp from header:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("timestamp expected but doesn't exist in header")
	}
	if l, ok := header["location"]; ok {
		fmt.Printf("location from header:\n")
		for i, e := range l {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("location expected but doesn't exist in header")
	}

}

func checkTrailer(trailer metadata.MD) {
	if t, ok := trailer["timestamp"]; ok {
		fmt.Printf("timestamp from trailer:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("timestamp expected but doesn't exist in trailer")
	}
}

func callUnaryEcho(client pb.EchoClient, message string) {
	fmt.Printf("--- unary ---\n")
	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	ctx = metadata.NewOutgoingContext(ctx, md)
	defer cancel()

	var header, trailer metadata.MD

	resp, err := client.UnaryEcho(
		ctx,
		&pb.EchoRequest{Message: message},
		grpc.Header(&header), // Similar to json
		grpc.Trailer(&trailer),
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

func makeRPCs(cc *grpc.ClientConn, n int) {
	hwc := pb.NewEchoClient(cc)
	for i := 0; i < n; i++ {
		callUnaryEcho(hwc, "this is examples/load_balancing")
	}
}

func getCustomCreds() (credentials.PerRPCCredentials, credentials.TransportCredentials) {
	perRPCCreds := oauth.NewOauthAccess(fetchToken())
	tspCreds, err := credentials.NewClientTLSFromFile(data.Path("x509/ca_cert.pem"), "x.test.example.com")
	if err != nil {
		log.Fatalf("failed to load credentials: %v", err)
	}
	return perRPCCreds, tspCreds
}

func dialNormal() *grpc.ClientConn {
	// Manual resolver configuration
	r := manual.NewBuilderWithScheme("whatever")
	r.InitialState(resolver.State{
		Addresses: []resolver.Address{
			{Addr: "localhost:50051"},
			{Addr: "localhost:50052"},
		},
	})
	address := fmt.Sprintf("%s:///unused", r.Scheme())
	// cred config
	perRPCCreds, tspCreds := getCustomCreds()
	// opts
	opts := []grpc.DialOption{
		grpc.WithPerRPCCredentials(perRPCCreds),
		// Bypass the TLS check, but this is not a good idea :)
		// grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(tspCreds),
		grpc.WithBlock(),
		grpc.WithResolvers(r), // block until underlying connection is up
		grpc.WithDefaultServiceConfig(servinceConfig),
		grpc.WithUnaryInterceptor(unaryInterceptor),
		grpc.WithStreamInterceptor(streamInterceptor),
	}
	// conn, err := grpc.Dial(*addr, opts...)
	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return conn
}

func dialPickFirst() *grpc.ClientConn {
	perRPCCreds, tspCreds := getCustomCreds()
	// "pick_first" is the default, so there's no need to set the load balancing policy.
	pickFirstConn, err := grpc.Dial(
		fmt.Sprintf("%s:///%s", "custom", "lb.custom.grpc.io"),
		grpc.WithPerRPCCredentials(perRPCCreds),
		grpc.WithTransportCredentials(tspCreds),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return pickFirstConn
}

func dialRoundRobin() *grpc.ClientConn {
	perRPCCreds, tspCreds := getCustomCreds()

	roundrobinConn, err := grpc.Dial(
		// fmt.Sprintf("%s:///%s", CustomScheme, CustomServiceName),
		fmt.Sprintf("%s:///%s", "custom", "lb.custom.grpc.io"),
		// fmt.Sprintf("%s:///%s", exampleScheme, exampleServiceName),

		grpc.WithPerRPCCredentials(perRPCCreds),
		grpc.WithTransportCredentials(tspCreds),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
	)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	return roundrobinConn
}

func main() {
	flag.Parse()

	// conn := dialNormal()
	conn := dialRoundRobin()
	// conn := dialPickFirst()
	defer conn.Close()

	ecClient := pb.NewEchoClient(conn)
	callUnaryEcho(ecClient, "hello world")
	callBidiStreamingEcho(ecClient, "hello world bidirection")
	callServerStreamingEcho(ecClient, "hello world server")
	callClientStreamingEcho(ecClient, "hello world client")

	// callUnaryWithHealthConfig(ecClient)

	// streamWithCancel(ecClient)

	// callUnaryEcho(ecClient, "hello world")

	// callUnaryWithGzip(ecClient)

	// callUnaryEchoWithErrorQuota(ecClient)
}
