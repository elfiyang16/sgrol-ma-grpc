package main

import (
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

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
