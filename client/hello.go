package main

import (
	"context"
	"fmt"
	"log"
	"time"

	hwpb "github.com/elfiyang16/sgrol-ma/proto/github.com/elfiyang16/sgrol-ma/proto/hello"
)

func callSayHello(c hwpb.GreeterClient, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &hwpb.HelloRequest{Name: name})
	if err != nil {
		log.Fatalf("client.SayHello(_) = _, %v", err)
	}
	fmt.Println("Greeting: ", r.Message)
}
