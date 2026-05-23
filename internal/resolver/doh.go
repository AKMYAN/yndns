package resolver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/miekg/dns"
)

// DoHResolver uses DNS-over-HTTPS.
type DoHResolver struct {
	Endpoint string // e.g., "https://cloudflare-dns.com/dns-query"
}

func (r *DoHResolver) Resolve(ctx context.Context, domain string) ([]string, error) {
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
			reply, err := r.query(ctx, domain, qt)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			ips = append(ips, extractIPs(reply)...)
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

func (r *DoHResolver) query(ctx context.Context, domain string, qtype uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	packed, err := m.Pack()
	if err != nil {
		return nil, fmt.Errorf("pack query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, bytes.NewReader(packed))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var reply dns.Msg
	if err := reply.Unpack(body); err != nil {
		return nil, fmt.Errorf("unpack response: %w", err)
	}

	return &reply, nil
}
