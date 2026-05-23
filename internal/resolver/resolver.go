package resolver

import (
	"context"
	"fmt"
	"net"
)

// Resolver resolves domain names to IP addresses.
type Resolver interface {
	Resolve(ctx context.Context, domain string) ([]string, error)
}

// SystemResolver uses the system DNS resolver.
type SystemResolver struct{}

func (r *SystemResolver) Resolve(ctx context.Context, domain string) ([]string, error) {
	resolver := net.Resolver{}
	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("system dns lookup: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP address found for %s", domain)
	}
	return ips, nil
}
