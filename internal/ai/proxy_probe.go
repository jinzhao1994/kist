package ai

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

const proxyTestURL = "https://ipwho.is/?fields=success,ip,country,region,city"

// ProxyTestResult is an ephemeral diagnostic, never archive or configuration data.
type ProxyTestResult struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Region    string `json:"region"`
	City      string `json:"city"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Error     string `json:"error" enum:",connection,timeout,rate_limited,service"`
}

// TestProxy queries a fixed HTTPS endpoint through exactly the selected proxy.
// Diagnostic failures are values so callers never expose transport credentials.
func TestProxy(ctx context.Context, proxy Proxy) ProxyTestResult {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := ProxyHTTPClient(&proxy, 15*time.Second)
	if err != nil {
		return ProxyTestResult{Error: "connection"}
	}
	defer client.CloseIdleConnections()
	return queryProxyExit(ctx, client)
}

func queryProxyExit(ctx context.Context, client *http.Client) ProxyTestResult {
	started := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, proxyTestURL, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil || isTimeout(err) {
			return ProxyTestResult{Error: "timeout"}
		}
		return ProxyTestResult{Error: "connection"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return ProxyTestResult{Error: "rate_limited"}
	}
	if resp.StatusCode != http.StatusOK {
		return ProxyTestResult{Error: "service"}
	}
	// Reject large or malformed replies instead of retaining arbitrary service data.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024+1))
	if err != nil {
		if ctx.Err() != nil || isTimeout(err) {
			return ProxyTestResult{Error: "timeout"}
		}
		return ProxyTestResult{Error: "service"}
	}
	var reply struct {
		Success bool `json:"success"`
		ProxyTestResult
	}
	if len(body) > 16*1024 || json.Unmarshal(body, &reply) != nil || !reply.Success || net.ParseIP(reply.IP) == nil || len(reply.Country) > 256 || len(reply.Region) > 256 || len(reply.City) > 256 {
		return ProxyTestResult{Error: "service"}
	}
	return ProxyTestResult{IP: reply.IP, Country: reply.Country, Region: reply.Region, City: reply.City, ElapsedMS: time.Since(started).Milliseconds()}
}

func isTimeout(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}
