package main

// for multiplex

import (
	"context"

	hwpb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/hello"
)

type hwServer struct {
	hwpb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *hwServer) SayHello(ctx context.Context, in *hwpb.HelloRequest) (*hwpb.HelloReply, error) {
	return &hwpb.HelloReply{Message: "Hello " + in.Name}, nil
}
