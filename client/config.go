package main

import (
	"flag"
	"time"

	"google.golang.org/grpc/keepalive"
)

const (
	fallbackToken   = "some-secret-token"
	timestampFormat = time.StampNano // "Jan _2 15:04:05.000"
	streamingCount  = 10
)

var addr = flag.String("addr", "localhost:50051", "http service address")

var retryPolicy = `{
		"methodConfig": [{
		"name": [{"service": "echo.Echo", "method": "UnaryEcho"}],
		"waitForReady": true,
		"retryPolicy": {
			"MaxAttempts": 4,
			"InitialBackoff": ".01s",
			"MaxBackoff": ".01s",
			"BackoffMultiplier": 1.0,
			"RetryableStatusCodes": [ "UNAVAILABLE" ]
		}
	}]
}
`

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
