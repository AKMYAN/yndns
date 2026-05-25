package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
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

const maxConcurrentEnrichments = 8

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
	if err := validateConfig(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
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
	if len(results) > 0 {
		formatter.PrintResults(os.Stdout, results)
	}
	return err
}

func enrichAll(ctx context.Context, enr enricher.Enricher, ips []string) ([]*enricher.Result, error) {
	type job struct {
		idx int
		ip  string
	}
	type outcome struct {
		idx int
		res *enricher.Result
		err error
	}

	jobs := make(chan job, len(ips))
	for i, ip := range ips {
		jobs <- job{idx: i, ip: ip}
	}
	close(jobs)

	outcomes := make(chan outcome, len(ips))
	workerCount := min(len(ips), maxConcurrentEnrichments)
	for range workerCount {
		go func() {
			for item := range jobs {
				res, err := enr.Enrich(ctx, item.ip)
				if err != nil {
					err = fmt.Errorf("enrich %s: %w", item.ip, err)
				} else if res == nil {
					err = fmt.Errorf("enrich %s: provider returned no result", item.ip)
				}
				outcomes <- outcome{idx: item.idx, res: res, err: err}
			}
		}()
	}

	indexed := make([]*enricher.Result, len(ips))
	failures := make([]error, len(ips))
	for range ips {
		outcome := <-outcomes
		if outcome.err != nil {
			failures[outcome.idx] = outcome.err
			continue
		}
		indexed[outcome.idx] = outcome.res
	}

	results := make([]*enricher.Result, 0, len(ips))
	errs := make([]error, 0)
	for i, result := range indexed {
		if result != nil {
			results = append(results, result)
		}
		if failures[i] != nil {
			errs = append(errs, failures[i])
		}
	}
	return results, errors.Join(errs...)
}

func loadConfig() error {
	cfg = Config{}
	for _, path := range configPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if configFile == "" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read %s: %w", path, err)
		}
		return yaml.Unmarshal(data, &cfg)
	}
	return nil // config file is optional
}

func validateConfig() error {
	if strings.TrimSpace(cfg.Token) == "" {
		return fmt.Errorf("IPInfo token is required; set token in config.yaml or use --config")
	}
	return nil
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
