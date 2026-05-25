package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yndns/internal/enricher"
)

func TestLoadConfigReturnsExplicitPathError(t *testing.T) {
	previousConfigFile, previousConfig := configFile, cfg
	t.Cleanup(func() {
		configFile, cfg = previousConfigFile, previousConfig
	})

	configFile = filepath.Join(t.TempDir(), "missing.yaml")
	err := loadConfig()
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "missing.yaml") {
		t.Fatalf("loadConfig error = %v, want missing explicit config path", err)
	}
}

func TestValidateConfigRequiresToken(t *testing.T) {
	previousConfig := cfg
	t.Cleanup(func() { cfg = previousConfig })

	cfg.Token = "  "
	if err := validateConfig(); err == nil {
		t.Fatal("validateConfig returned no error for an empty token")
	}
	cfg.Token = "token"
	if err := validateConfig(); err != nil {
		t.Fatalf("validateConfig: %v", err)
	}
}

func TestEnrichAllLimitsConcurrencyAndKeepsSuccessfulResults(t *testing.T) {
	release := make(chan struct{})
	provider := &controlledEnricher{release: release, failIP: "ip-3"}
	ips := make([]string, 20)
	for i := range ips {
		ips[i] = fmt.Sprintf("ip-%d", i)
	}

	type response struct {
		results []*enricher.Result
		err     error
	}
	done := make(chan response, 1)
	go func() {
		results, err := enrichAll(context.Background(), provider, ips)
		done <- response{results: results, err: err}
	}()

	deadline := time.After(2 * time.Second)
	for provider.active.Load() < maxConcurrentEnrichments {
		select {
		case <-deadline:
			t.Fatal("workers did not reach the configured concurrency limit")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := provider.max.Load(); got != maxConcurrentEnrichments {
		t.Fatalf("maximum active enrichments = %d, want %d", got, maxConcurrentEnrichments)
	}
	close(release)

	result := <-done
	if result.err == nil || !strings.Contains(result.err.Error(), "ip-3") {
		t.Fatalf("enrichAll error = %v, want failure for ip-3", result.err)
	}
	if len(result.results) != len(ips)-1 {
		t.Fatalf("enrichAll returned %d successes, want %d", len(result.results), len(ips)-1)
	}
	if result.results[3].IP != "ip-4" {
		t.Fatalf("successful results lost input order: result[3] = %q", result.results[3].IP)
	}
}

type controlledEnricher struct {
	release chan struct{}
	failIP  string
	active  atomic.Int32
	max     atomic.Int32
}

func (e *controlledEnricher) Enrich(ctx context.Context, ip string) (*enricher.Result, error) {
	active := e.active.Add(1)
	for {
		max := e.max.Load()
		if active <= max || e.max.CompareAndSwap(max, active) {
			break
		}
	}
	defer e.active.Add(-1)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.release:
	}
	if ip == e.failIP {
		return nil, errors.New("lookup failed")
	}
	return &enricher.Result{IP: ip}, nil
}
