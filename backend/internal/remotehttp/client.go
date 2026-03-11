package remotehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewClient returns an HTTP client for remote fetches that blocks requests to
// localhost and private network targets.
func NewClient(timeout time.Duration) *http.Client {
	baseDialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if err := validateHost(ctx, host); err != nil {
			return nil, err
		}
		return baseDialer.DialContext(ctx, network, addr)
	}

	return &http.Client{
		Timeout: timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return ValidateURL(req.URL.String())
		},
	}
}

// ValidateURL ensures the URL uses http(s) and does not target internal hosts.
func ValidateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("only http and https URLs are allowed")
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return errors.New("url host is required")
	}
	return validateHost(context.Background(), host)
}

func validateHost(ctx context.Context, host string) error {
	if strings.EqualFold(host, "localhost") {
		return errors.New("localhost is not allowed")
	}

	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("host did not resolve to an IP")
	}
	for _, ip := range ips {
		if err := validateIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func validateIP(ip net.IP) error {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("ip %s is not allowed", ip.String())
	}
	return nil
}
