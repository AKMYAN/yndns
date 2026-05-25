package resolver

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestNormalizeServer(t *testing.T) {
	tests := map[string]string{
		"8.8.8.8":              "8.8.8.8:53",
		"8.8.8.8:5353":         "8.8.8.8:5353",
		"2001:4860:4860::8888": "[2001:4860:4860::8888]:53",
		"[2001:db8::1]:5353":   "[2001:db8::1]:5353",
		"dns.example":          "dns.example:53",
	}

	for input, expected := range tests {
		got, err := normalizeServer(input)
		if err != nil {
			t.Fatalf("normalizeServer(%q): %v", input, err)
		}
		if got != expected {
			t.Errorf("normalizeServer(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestNormalizeServerRejectsMalformedIPv6Port(t *testing.T) {
	if _, err := normalizeServer("[2001:db8::1"); err == nil {
		t.Fatal("normalizeServer accepted an incomplete bracketed IPv6 address")
	}
}

func TestQueryServerRetriesTruncatedUDPOverTCP(t *testing.T) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, request *dns.Msg) {
		reply := new(dns.Msg)
		reply.SetReply(request)
		if w.LocalAddr().Network() == "udp" {
			reply.Truncated = true
			_ = w.WriteMsg(reply)
			return
		}
		reply.Answer = append(reply.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: request.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.ParseIP("192.0.2.10"),
		})
		_ = w.WriteMsg(reply)
	})
	address := startUDPAndTCPServer(t, handler)

	reply, err := queryServer(context.Background(), address, "example.test", dns.TypeA)
	if err != nil {
		t.Fatalf("queryServer: %v", err)
	}
	ips := extractIPs(reply)
	if len(ips) != 1 || ips[0] != "192.0.2.10" {
		t.Fatalf("extractIPs() = %v, want [192.0.2.10]", ips)
	}
}

func TestQueryServerHonorsContextCancellation(t *testing.T) {
	received := make(chan struct{}, 1)
	release := make(chan struct{})
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, request *dns.Msg) {
		received <- struct{}{}
		<-release
	})

	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &dns.Server{PacketConn: packetConn, Handler: handler}
	go func() { _ = server.ActivateAndServe() }()
	t.Cleanup(func() {
		close(release)
		_ = server.Shutdown()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = queryServer(ctx, packetConn.LocalAddr().String(), "example.test", dns.TypeA)
	var timeoutErr net.Error
	if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &timeoutErr) || !timeoutErr.Timeout()) {
		t.Fatalf("queryServer error = %v, want a context-driven timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("queryServer returned after %s, want prompt context cancellation", elapsed)
	}
	select {
	case <-received:
	default:
		t.Fatal("DNS server did not receive the query before cancellation")
	}
}

func startUDPAndTCPServer(t *testing.T, handler dns.Handler) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	packetConn, err := net.ListenPacket("udp", listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	tcpServer := &dns.Server{Listener: listener, Handler: handler}
	udpServer := &dns.Server{PacketConn: packetConn, Handler: handler}
	go func() { _ = tcpServer.ActivateAndServe() }()
	go func() { _ = udpServer.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = udpServer.Shutdown()
		_ = tcpServer.Shutdown()
	})
	return listener.Addr().String()
}
