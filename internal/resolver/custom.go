package resolver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// CustomResolver queries a specific DNS server via UDP/TCP.
type CustomResolver struct {
	Server string // e.g., "8.8.8.8" or "8.8.8.8:53"
}

func (r *CustomResolver) Resolve(ctx context.Context, domain string) ([]string, error) {
	server := r.Server
	if !strings.Contains(server, ":") {
		server = server + ":53"
	}

	c := new(dns.Client)

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
			m := new(dns.Msg)
			m.SetQuestion(dns.Fqdn(domain), qt)
			m.RecursionDesired = true

			type result struct {
				resp *dns.Msg
				err  error
			}

			ch := make(chan result, 1)
			go func() {
				resp, _, err := c.Exchange(m, server)
				ch <- result{resp, err}
			}()

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
			case r := <-ch:
				if r.err != nil {
					errCh <- fmt.Errorf("dns query %s: %w", server, r.err)
					return
				}
				mu.Lock()
				ips = append(ips, extractIPs(r.resp)...)
				mu.Unlock()
			}
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
