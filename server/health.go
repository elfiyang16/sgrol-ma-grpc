package main

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func runHealthSvr(server *grpc.Server) {
	// Register health server
	healthcheckSvr := health.NewServer() // include statusMap and updates (map[string]map[healthgrpc.Health_WatchServer]chan healthpb.HealthCheckResponse_ServingStatus))
	healthgrpc.RegisterHealthServer(server, healthcheckSvr)

	// start healthserver in a subroutine
	go func() {
		next := healthpb.HealthCheckResponse_SERVING
		// Here manually set the health status of the server.
		for {
			healthcheckSvr.SetServingStatus(system, next)
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
