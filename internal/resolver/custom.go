package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// CustomResolver queries a specific DNS server via UDP/TCP.
type CustomResolver struct {
	Server string // e.g., "8.8.8.8" or "8.8.8.8:53"
}

func (r *CustomResolver) Resolve(ctx context.Context, domain string) ([]string, error) {
	server, err := normalizeServer(r.Server)
	if err != nil {
		return nil, err
	}

	var (
		ips []string
		mu  sync.Mutex
		wg  sync.WaitGroup
	)

	errCh := make(chan error, 2)

	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA} {
		wg.Add(1)
		go func(qt uint16) {
			defer wg.Done()
			resp, err := queryServer(ctx, server, domain, qt)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			ips = append(ips, extractIPs(resp)...)
			mu.Unlock()
		}(qtype)
	}

	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no records found for %s", domain)
	}
	return ips, nil
}

func normalizeServer(server string) (string, error) {
	if server == "" {
		return "", fmt.Errorf("DNS server cannot be empty")
	}
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server, nil
	}
	if net.ParseIP(server) != nil {
		return net.JoinHostPort(server, "53"), nil
	}
	if strings.HasPrefix(server, "[") && strings.HasSuffix(server, "]") {
		unbracketed := server[1 : len(server)-1]
		if net.ParseIP(unbracketed) != nil {
			return net.JoinHostPort(unbracketed, "53"), nil
		}
	}
	if strings.Contains(server, ":") {
		return "", fmt.Errorf("invalid DNS server %q: specify an IPv6 port as [address]:port", server)
	}
	return net.JoinHostPort(server, "53"), nil
}

func queryServer(ctx context.Context, server, domain string, qtype uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	client := &dns.Client{Net: "udp"}
	resp, _, err := client.ExchangeContext(ctx, m, server)
	if err != nil {
		return nil, fmt.Errorf("dns query %s: %w", server, err)
	}
	if resp.Truncated {
		client.Net = "tcp"
		resp, _, err = client.ExchangeContext(ctx, m, server)
		if err != nil {
			return nil, fmt.Errorf("dns tcp retry %s: %w", server, err)
		}
	}
	if resp.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns query %s returned %s", server, dns.RcodeToString[resp.Rcode])
	}
	return resp, nil
}

func extractIPs(msg *dns.Msg) []string {
	var ips []string
	for _, ans := range msg.Answer {
		switch r := ans.(type) {
		case *dns.A:
			ips = append(ips, r.A.String())
		case *dns.AAAA:
			ips = append(ips, r.AAAA.String())
		}
	}
	return ips
}
