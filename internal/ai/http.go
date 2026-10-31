package ai

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient applies only the AI transport policy. Empty inherits process proxy settings.
func HTTPClient(proxyAddress string, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyAddress != "" {
		address, err := url.Parse(proxyAddress)
		if err != nil || !ValidNetworkHost(address.Hostname()) || address.Opaque != "" || address.Fragment != "" || address.RawQuery != "" || (address.Path != "" && address.Path != "/") {
			return nil, errors.New("invalid AI proxy address")
		}
		if strings.HasSuffix(address.Host, ":") {
			return nil, errors.New("invalid AI proxy port")
		}
		if port := address.Port(); port != "" {
			number, err := strconv.Atoi(port)
			if err != nil || number < 1 || number > 65535 {
				return nil, errors.New("invalid AI proxy port")
			}
		}
		switch address.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return nil, errors.New("AI proxy must use http, https or socks5")
		}
		transport.Proxy = http.ProxyURL(address)
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

// ValidNetworkHost checks a hostname or IP literal without a scheme or port.
func ValidNetworkHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
