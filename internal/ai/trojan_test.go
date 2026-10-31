package ai

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTrojanHeaderAddressFamilies(t *testing.T) {
	for _, tc := range []struct{ destination, suffix string }{
		{"api.example:443", "01030b6170692e6578616d706c6501bb0d0a"},
		{"192.0.2.1:80", "0101c000020100500d0a"},
		{"[2001:db8::1]:8443", "010420010db800000000000000000000000120fb0d0a"},
	} {
		raw, err := trojanHeader("synthetic-password", tc.destination)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) < 58 || string(raw[56:58]) != "\r\n" || hex.EncodeToString(raw[58:]) != tc.suffix {
			t.Fatalf("invalid address encoding for %s", tc.destination)
		}
		// SHA-224 is a fixed 56-character lowercase hex credential, not plaintext.
		if _, err := hex.DecodeString(string(raw[:56])); err != nil || strings.Contains(string(raw), "synthetic-password") {
			t.Fatal("invalid credential encoding")
		}
	}
	for _, destination := range []string{"bad", ":443", "api.example:0", "api.example:65536", strings.Repeat("a", 256) + ":443"} {
		if _, err := trojanHeader("test", destination); err == nil {
			t.Fatalf("accepted %s", destination)
		}
	}
}

func TestTrojanVerifiedTunnel(t *testing.T) {
	certificate := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	certificate.Close()
	roots := x509.NewCertPool()
	roots.AddCert(certificate.Certificate())
	listener, err := tls.Listen("tcp", "127.0.0.1:0", certificate.TLS)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	proxy := Proxy{Type: ProxyTrojan, Host: host, Port: port, Password: "synthetic-secret"}
	header, _ := trojanHeader(proxy.Password, "provider.invalid:443")
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		got := make([]byte, len(header))
		_, err = io.ReadFull(conn, got)
		if err != nil {
			done <- err
			return
		}
		if string(got) != string(header) {
			done <- io.ErrUnexpectedEOF
			return
		}
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err == nil && req.Host != "provider.invalid" {
			err = io.ErrUnexpectedEOF
		}
		if err == nil {
			_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 6\r\n\r\ntunnel")
		}
		done <- err
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := dialTrojanTLS(ctx, header, proxy, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: host})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err = io.WriteString(conn, "GET /models HTTP/1.1\r\nHost: provider.invalid\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "tunnel" {
		t.Fatal("unexpected tunnel response")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTrojanRejectsUntrustedTLSAndCanceledDial(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	host, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	proxy := Proxy{Type: ProxyTrojan, Host: host, Port: port, Password: "synthetic-secret"}
	if conn, err := dialTrojan(context.Background(), "tcp", "provider.invalid:443", proxy); err == nil {
		conn.Close()
		t.Fatal("accepted untrusted certificate")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if conn, err := dialTrojan(ctx, "tcp", "provider.invalid:443", proxy); err == nil {
		conn.Close()
		t.Fatal("ignored canceled context")
	}
}

func TestSelectedProxyTransportDoesNotUseEnvironment(t *testing.T) {
	client, err := ProxyHTTPClient(nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("direct account inherits process proxy")
	}
	for _, p := range []Proxy{{Type: ProxyHTTP}, {Type: "unknown"}, {Type: ProxyTrojan, Host: "host", Port: 443}} {
		if _, err := ProxyHTTPClient(&p, time.Second); err == nil {
			t.Fatal("accepted incomplete proxy")
		}
	}
}
