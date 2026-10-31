package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// ProxyHTTPClient uses exactly the selected transport. Nil means direct,
// independent of environment proxies. It opens no listener or child process.
func ProxyHTTPClient(proxy *Proxy, timeout time.Duration) (*http.Client, error) {
	if proxy != nil && proxy.Type == ProxyHTTP {
		if proxy.URL == "" {
			return nil, errors.New("HTTP proxy URL is required")
		}
		return HTTPClient(proxy.URL, timeout)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if proxy != nil {
		if proxy.Type != ProxyTrojan || proxy.Host == "" || proxy.Port < 1 || proxy.Port > 65535 || proxy.Password == "" {
			return nil, errors.New("invalid Trojan proxy configuration")
		}
		p := *proxy
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialTrojan(ctx, network, address, p)
		}
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

// dialTrojan implements the published Trojan TCP CONNECT protocol over verified
// TLS. The destination name travels inside TLS, so DNS is resolved remotely.
func dialTrojan(ctx context.Context, network, address string, proxy Proxy) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, errors.New("Trojan supports TCP connections only")
	}
	header, err := trojanHeader(proxy.Password, address)
	if err != nil {
		return nil, err
	}
	name := proxy.ServerName
	if name == "" {
		name = proxy.Host
	}
	return dialTrojanTLS(ctx, header, proxy, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: name})
}

func dialTrojanTLS(ctx context.Context, header []byte, proxy Proxy, tlsConfig *tls.Config) (net.Conn, error) {
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second},
		Config:    tlsConfig,
	}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(proxy.Host, strconv.Itoa(proxy.Port)))
	if err != nil {
		return nil, errors.New("Trojan TLS connection failed")
	}
	// Cancellation must also interrupt an authenticated write, after TLS setup.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	deadline := time.Now().Add(30 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = conn.SetWriteDeadline(deadline)
	_, err = io.Copy(conn, bytes.NewReader(header))
	stopped := stop()
	if err != nil || !stopped || ctx.Err() != nil {
		_ = conn.Close()
		return nil, errors.New("Trojan connection canceled or authentication write failed")
	}
	_ = conn.SetWriteDeadline(time.Time{})
	return conn, nil
}

func trojanHeader(password, address string) ([]byte, error) {
	host, portString, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, errors.New("invalid Trojan destination")
	}
	port, err := strconv.Atoi(portString)
	if err != nil || port < 1 || port > 65535 {
		return nil, errors.New("invalid Trojan destination port")
	}
	digest := sha256.Sum224([]byte(password))
	header := []byte(hex.EncodeToString(digest[:]) + "\r\n")
	header = append(header, 1) // TCP CONNECT
	ip := net.ParseIP(host)
	switch {
	case ip != nil && ip.To4() != nil:
		header = append(header, 1)
		header = append(header, ip.To4()...)
	case ip != nil:
		header = append(header, 4)
		header = append(header, ip.To16()...)
	default:
		if len(host) > 255 {
			return nil, errors.New("Trojan destination name is too long")
		}
		header = append(header, 3, byte(len(host)))
		header = append(header, host...)
	}
	header = append(header, byte(port>>8), byte(port), '\r', '\n')
	return header, nil
}
