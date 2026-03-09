package runner

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/nickromney/website-testing/internal/spec"
)

func TestRunHTTPStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))
	t.Cleanup(srv.Close)

	r, err := New(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	step := spec.Step{
		Name: "one",
		Kind: spec.KindHTTP,
		Request: spec.Request{
			Method: "GET",
			URL:    srv.URL,
		},
		Expect: spec.Expect{
			Status:       http.StatusOK,
			BodyContains: []string{"hello"},
			BodyAbsent:   []string{"nope"},
			HeaderHas:    []string{"Content-Type: text/plain"},
		},
	}

	res := r.RunStep(context.Background(), step)
	if !res.Passed {
		t.Fatalf("expected pass, errors=%v", res.Errors)
	}
}

func TestRunDNSStep(t *testing.T) {
	handler := dns.NewServeMux()
	handler.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {
		msg := new(dns.Msg)
		msg.SetReply(req)
		for _, q := range req.Question {
			if q.Qtype == dns.TypeA {
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP("127.0.0.1"),
				})
			}
		}
		_ = w.WriteMsg(msg)
	})

	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	server := &dns.Server{PacketConn: packetConn, Handler: handler}
	go func() {
		_ = server.ActivateAndServe()
	}()
	t.Cleanup(func() {
		_ = server.Shutdown()
		_ = packetConn.Close()
	})

	r, err := New(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	step := spec.Step{
		Name: "dns",
		Kind: spec.KindDNS,
		DNS: spec.DNSQuery{
			Name:   "example.test",
			Type:   "A",
			Server: packetConn.LocalAddr().String(),
		},
		Expect: spec.Expect{
			AnswerContains: []string{"127.0.0.1"},
		},
	}

	res := r.RunStep(context.Background(), step)
	if !res.Passed {
		t.Fatalf("expected DNS pass, errors=%v", res.Errors)
	}
	if len(res.DNSAnswers) != 1 || res.DNSAnswers[0] != "127.0.0.1" {
		t.Fatalf("unexpected answers: %#v", res.DNSAnswers)
	}
}

func TestRunTLSStep(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	r, err := New(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	minDays := 0
	step := spec.Step{
		Name: "tls",
		Kind: spec.KindTLS,
		TLS: spec.TLSProbe{
			Address: srv.Listener.Addr().String(),
		},
		Expect: spec.Expect{
			DaysRemainingAtLeast: &minDays,
		},
	}

	res := r.RunStep(context.Background(), step)
	if !res.Passed {
		t.Fatalf("expected TLS pass, errors=%v", res.Errors)
	}
	if res.TLSNotAfter.IsZero() {
		t.Fatalf("expected TLS notAfter to be populated")
	}
}

func TestRunTCPStep(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
		close(accepted)
	}()

	r, err := New(2 * time.Second)
	if err != nil {
		t.Fatal(err)
	}

	step := spec.Step{
		Name: "tcp",
		Kind: spec.KindTCP,
		TCP: spec.TCPProbe{
			Address: listener.Addr().String(),
		},
	}

	res := r.RunStep(context.Background(), step)
	if !res.Passed {
		t.Fatalf("expected TCP pass, errors=%v", res.Errors)
	}

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tcp accept")
	}
}
