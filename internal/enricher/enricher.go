package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Result holds enriched IP information.
type Result struct {
	IP       string
	ASN      string
	ASName   string
	ASDomain string
	Country  string
}

// Enricher is the interface for IP enrichment providers.
// Implement this to support different API backends (IPinfo, IP-API, etc.).
type Enricher interface {
	Enrich(ctx context.Context, ip string) (*Result, error)
}

// IPInfoEnricher implements Enricher using ipinfo.io.
type IPInfoEnricher struct {
	Token      string
	HTTPClient *http.Client
}

// NewIPInfo creates a new IPInfo enricher.
func NewIPInfo(token string) *IPInfoEnricher {
	return &IPInfoEnricher{
		Token:      token,
		HTTPClient: &http.Client{},
	}
}

type ipinfoResponse struct {
	IP       string `json:"ip"`
	ASN      string `json:"asn"`
	ASName   string `json:"as_name"`
	ASDomain string `json:"as_domain"`
	Country  string `json:"country"`
}

func (e *IPInfoEnricher) Enrich(ctx context.Context, ip string) (*Result, error) {
	url := fmt.Sprintf("https://api.ipinfo.io/lite/%s?token=%s", ip, e.Token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	var apiResp ipinfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &Result{
		IP:       apiResp.IP,
		ASN:      apiResp.ASN,
		ASName:   apiResp.ASName,
		ASDomain: apiResp.ASDomain,
		Country:  apiResp.Country,
	}, nil
}
