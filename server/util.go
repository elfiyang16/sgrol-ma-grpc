package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"

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

// logger is to mock a sophisticated logging system. To simplify the example, we just print out the content.
func logger(format string, a ...interface{}) {
	fmt.Printf("LOG:\t"+format+"\n", a...)
}
