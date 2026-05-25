# yndns

A CLI tool for DNS resolution and IP/ASN enrichment.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/AKMYAN/yndns/main/install-binary.sh | bash
```

Or build from source:

```bash
go build -o ~/.local/bin/yndns .
```

## Configure

Place your [IPInfo](https://ipinfo.io) token in `~/.config/yndns/config.yaml`:

```yaml
token: "your-token"
```

## Usage

```bash
# Query an IP directly
yndns 8.8.8.8

# Resolve a domain with system DNS
yndns example.com

# Custom DNS server
yndns -s 8.8.8.8 example.com

# DNS-over-HTTPS shortcuts
yndns --cf example.com     # Cloudflare DoH
yndns --goo example.com    # Google DoH

# Custom DoH endpoint
yndns --doh https://dns.quad9.net/dns-query example.com
```

## Output

```
104.16.133.229  AS13335  Cloudflare, Inc.  cloudflare.com  United States
```

Fields: `IP  ASN  AS_NAME  AS_Domain  Country`

## Outlook

Support MX and NS Record.