package resolver

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestDoHResolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		query, ok := readDNSQuery(t, request)
		if !ok {
			http.Error(w, "invalid query", http.StatusBadRequest)
			return
		}
		reply := new(dns.Msg)
		reply.SetReply(query)
		switch query.Question[0].Qtype {
		case dns.TypeA:
			reply.Answer = append(reply.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET},
				A:   net.ParseIP("192.0.2.2"),
			})
		case dns.TypeAAAA:
			reply.Answer = append(reply.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET},
				AAAA: net.ParseIP("2001:db8::2"),
			})
		}
		writeDNSReply(t, w, reply)
	}))
	defer server.Close()

	resolver := &DoHResolver{Endpoint: server.URL, HTTPClient: server.Client()}
	ips, err := resolver.Resolve(context.Background(), "example.test")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	got := map[string]bool{}
	for _, ip := range ips {
		got[ip] = true
	}
	if !got["192.0.2.2"] || !got["2001:db8::2"] {
		t.Fatalf("Resolve returned %v", ips)
	}
}

func TestDoHQueryRejectsInvalidResponses(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"http status": func(w http.ResponseWriter, request *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		},
		"oversized response": func(w http.ResponseWriter, request *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", maxDoHResponseBytes+1))
		},
		"dns rcode": func(w http.ResponseWriter, request *http.Request) {
			query, ok := readDNSQuery(t, request)
			if !ok {
				http.Error(w, "invalid query", http.StatusBadRequest)
				return
			}
			reply := new(dns.Msg)
			reply.SetRcode(query, dns.RcodeServerFailure)
			writeDNSReply(t, w, reply)
		},
	}

	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			resolver := &DoHResolver{Endpoint: server.URL, HTTPClient: server.Client()}
			if _, err := resolver.query(context.Background(), "example.test", dns.TypeA); err == nil {
				t.Fatal("query returned no error")
			}
		})
	}
}

func readDNSQuery(t *testing.T, request *http.Request) (*dns.Msg, bool) {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Errorf("read request: %v", err)
		return nil, false
	}
	query := new(dns.Msg)
	if err := query.Unpack(body); err != nil {
		t.Errorf("unpack request: %v", err)
		return nil, false
	}
	return query, true
}

func writeDNSReply(t *testing.T, w http.ResponseWriter, reply *dns.Msg) {
	t.Helper()
	body, err := reply.Pack()
	if err != nil {
		t.Errorf("pack reply: %v", err)
		http.Error(w, "failed to pack reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/dns-message")
	_, _ = w.Write(body)
}
