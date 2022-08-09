package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

func valid(authorization []string) bool {
	if len(authorization) < 1 {
		return false
	}
	token := strings.TrimPrefix(authorization[0], "Bearer ")
	return token == "some-secret-token-xxx" // ignore validation and just pretend to check against a string
}

func ensureValidToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errMissingMetadata
	}
	// Check the client's token to verify it's validity.
	if !valid(meta["authorization"]) {
		return nil, errInvalidToken
	}
	// If valid, let the request through.
	return handler(ctx, req)
}

func runHealthSvr(server *grpc.Server) {
	// Register health server
	healthcheck := health.NewServer() // include statusMap and updates (map[string]map[healthgrpc.Health_WatchServer]chan healthpb.HealthCheckResponse_ServingStatus))
	healthgrpc.RegisterHealthServer(server, healthcheck)

	// start healthserver in a subroutine
	go func() {
		next := healthpb.HealthCheckResponse_SERVING
		// Here manually set the health status of the server.
		for {
			healthcheck.SetServingStatus(system, next)
			// Simulate the changes in server health, take turns from unhealthy to healthy
			if next == healthpb.HealthCheckResponse_SERVING {
				next = healthpb.HealthCheckResponse_NOT_SERVING
			} else {
				next = healthpb.HealthCheckResponse_SERVING
			}
			time.Sleep(*sleep)
		}
	}()
}

// logger is to mock a sophisticated logging system. To simplify the example, we just print out the content.
func logger(format string, a ...interface{}) {
	fmt.Printf("LOG:\t"+format+"\n", a...)
}
