package ai

import (
	"errors"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient applies only the AI transport policy. Empty inherits process proxy settings.
func HTTPClient(proxyAddress string, timeout time.Duration) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyAddress != "" {
		address, err := url.Parse(proxyAddress)
		if err != nil || address.Hostname() == "" || address.Opaque != "" || address.Fragment != "" || address.RawQuery != "" || (address.Path != "" && address.Path != "/") {
			return nil, errors.New("invalid AI proxy address")
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
