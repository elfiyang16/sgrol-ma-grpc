package main

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func runRelectionSvr(s *grpc.Server) {
	reflection.Register(s)
}
