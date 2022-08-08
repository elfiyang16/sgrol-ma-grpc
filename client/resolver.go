package main

import "google.golang.org/grpc/resolver"

const (
	CustomScheme      = "custom"
	CustomServiceName = "lb.custom.grpc.io"
)

var addrs = []string{"localhost:50051", "localhost:50052"}

type CustomResolverBuilder struct{}

type CustomResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

// 	Build(target Target, cc ClientConn, opts BuildOptions) (Resolver, error)
func (c *CustomResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &CustomResolver{
		target: target,
		cc:     cc,
		addrsStore: map[string][]string{
			CustomServiceName: addrs,
		},
	}
	r.start()
	return r, nil
}

func (c *CustomResolverBuilder) Scheme() string {
	return CustomScheme
}

func (r *CustomResolver) start() {
	// get the backend server addresses ip+port
	addrStrs := r.addrsStore[r.target.Endpoint]
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[i] = resolver.Address{Addr: s}
	}
	// update the client conn with the cur resolver state
	r.cc.UpdateState(resolver.State{
		Addresses: addrs,
		//  ServiceConfig *serviceconfig.ParseResult
		//  Attributes *attributes.Attributes
	})
}

func (r *CustomResolver) ResolveNow(o resolver.ResolveNowOptions) {}

func (r *CustomResolver) Close() {}

func init() {
	resolver.Register(&CustomResolverBuilder{})
}
