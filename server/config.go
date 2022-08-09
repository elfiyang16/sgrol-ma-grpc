package main

import (
	"flag"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const (
	REAPEAT_COUNT   = 1000
	timestampFormat = time.StampNano
	streamingCount  = 10
)

var (
	// grpc's own error code
	errMissingMetadata = status.Errorf(codes.InvalidArgument, "missing metadata")
	errInvalidToken    = status.Errorf(codes.Unauthenticated, "invalid token")
)

var (
	// port  = flag.Int("port", 50051, "the port to serve on")
	addrs = []string{":50051", ":50052"}

	sleep = flag.Duration("sleep", 0, "duration between changes in health")

	system = "" // empty system means all systems
)

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
	PermitWithoutStream: true,            // allow pings even when there are no active streams
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle:     30 * time.Second, //  default value is infinity.
	MaxConnectionAge:      30 * time.Second, // default value is infinity.
	MaxConnectionAgeGrace: 5 * time.Second,  // default value is infinity.
	Time:                  10 * time.Second, // time to wait to ping client, default to 2 hours
	Timeout:               1 * time.Second,  // time to wait for ping ack before close conn, default to 20 second
}
