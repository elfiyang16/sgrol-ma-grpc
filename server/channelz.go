package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/channelz/service"
)

// run Channelz by registering the channelz service to a gRPC server
// it needs to be on an independent port and an independent server
func runChannelzSvr() {
	lis, err := net.Listen("tcp", ":10007")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()
	s := grpc.NewServer()
	service.RegisterChannelzServiceToServer(s)
	go s.Serve(lis)
	defer s.Stop()
}
