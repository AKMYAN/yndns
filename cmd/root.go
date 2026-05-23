package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"yndns/internal/enricher"
	"yndns/internal/formatter"
	"yndns/internal/resolver"
)

// Config holds the application configuration from config.yaml.
type Config struct {
	Token string `yaml:"token"`
}

var (
	configFile  string
	dnsServer   string
	dohEndpoint string
	cfDoh       bool
	gooDoh      bool
	cfg         Config
)

var rootCmd = &cobra.Command{
	Use:   "yndns [domain|ip]",
	Short: "DNS resolution and IP/ASN enrichment tool",
	Long: `yndns is a CLI tool for DNS resolution and IP/ASN information lookup.

It accepts a domain name or IP address and returns enriched information
including ASN, AS name, AS domain, and country for each resolved IP.`,
	Args: cobra.ExactArgs(1),
	RunE: run,
}

func run(cmd *cobra.Command, args []string) error {
	input := args[0]

	if err := loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	enr := enricher.NewIPInfo(cfg.Token)

	var ips []string

	if net.ParseIP(input) != nil {
		ips = []string{input}
	} else {
		var r resolver.Resolver
		switch {
		case dohURL() != "":
			r = &resolver.DoHResolver{Endpoint: dohURL()}
		case dnsServer != "":
			r = &resolver.CustomResolver{Server: dnsServer}
		default:
			r = &resolver.SystemResolver{}
		}

		var err error
		ips, err = r.Resolve(ctx, input)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", input, err)
		}
	}

	results, err := enrichAll(ctx, enr, ips)
	if err != nil {
		return err
	}

	formatter.PrintResults(os.Stdout, results)
	return nil
}

func enrichAll(ctx context.Context, enr enricher.Enricher, ips []string) ([]*enricher.Result, error) {
	type idxResult struct {
		idx int
		res *enricher.Result
	}

	results := make([]*enricher.Result, len(ips))
	ch := make(chan idxResult, len(ips))
	errCh := make(chan error, len(ips))

	for i, ip := range ips {
		go func(idx int, ipAddr string) {
			res, err := enr.Enrich(ctx, ipAddr)
			if err != nil {
				errCh <- fmt.Errorf("enrich %s: %w", ipAddr, err)
				return
			}
			ch <- idxResult{idx: idx, res: res}
		}(i, ip)
	}

	received := 0
	for received < len(ips) {
		select {
		case r := <-ch:
			results[r.idx] = r.res
			received++
		case err := <-errCh:
			return nil, err
		}
	}

	return results, nil
}

func loadConfig() error {
	for _, path := range configPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return yaml.Unmarshal(data, &cfg)
	}
	return nil // config file is optional
}

func configPaths() []string {
	if configFile != "" {
		return []string{configFile}
	}
	paths := []string{"config.yaml"}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, xdg+"/yndns/config.yaml")
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, home+"/.config/yndns/config.yaml")
	}
	return paths
}

func dohURL() string {
	if cfDoh {
		return "https://cloudflare-dns.com/dns-query"
	}
	if gooDoh {
		return "https://dns.google/dns-query"
	}
	return dohEndpoint
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")
	rootCmd.Flags().StringVarP(&dnsServer, "server", "s", "", "Custom DNS server (e.g., 8.8.8.8)")
	rootCmd.Flags().StringVar(&dohEndpoint, "doh", "", "DoH endpoint URL")
	rootCmd.Flags().BoolVar(&cfDoh, "cf", false, "Use Cloudflare DoH (https://cloudflare-dns.com/dns-query)")
	rootCmd.Flags().BoolVar(&gooDoh, "goo", false, "Use Google DoH (https://dns.google/dns-query)")
}
