/*
 * Copyright (c) Cherri
 */

// Package icloudshortcut resolves public iCloud Shortcut share links to the
// underlying unsigned Shortcut plist bytes. Apple's records endpoint is
// undocumented, so all dependency on it is deliberately isolated here.
package icloudshortcut

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxBytes = int64(20 << 20)
	DefaultTimeout  = 25 * time.Second
)

var shortcutIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{16,64}$`)

type Result struct {
	ID   string
	Name string
	Data []byte
}

type Resolver struct {
	Client         *http.Client
	MaxBytes       int64
	RecordsBaseURL string
}

type recordResponse struct {
	Fields struct {
		Name struct {
			Value string `json:"value"`
		} `json:"name"`
		Shortcut struct {
			Value struct {
				DownloadURL string `json:"downloadURL"`
			} `json:"value"`
		} `json:"shortcut"`
	} `json:"fields"`
}

func NewResolver() *Resolver {
	return &Resolver{Client: secureClient(DefaultTimeout), MaxBytes: DefaultMaxBytes, RecordsBaseURL: "https://www.icloud.com/shortcuts/api/records/"}
}

func CanonicalURL(raw string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil { return "", "", fmt.Errorf("invalid iCloud URL: %w", err) }
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "icloud.com") && !strings.EqualFold(parsed.Hostname(), "www.icloud.com") {
		return "", "", errors.New("Shortcut share URL must use https://www.icloud.com")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "shortcuts" || !shortcutIDPattern.MatchString(parts[1]) {
		return "", "", errors.New("invalid iCloud Shortcut share URL")
	}
	id := parts[1]
	return "https://www.icloud.com/shortcuts/" + id, id, nil
}

func (resolver *Resolver) Resolve(ctx context.Context, raw string) (*Result, error) {
	_, id, err := CanonicalURL(raw)
	if err != nil { return nil, err }
	client := resolver.Client
	if client == nil { client = secureClient(DefaultTimeout) }
	limit := resolver.MaxBytes
	if limit <= 0 { limit = DefaultMaxBytes }
	base := resolver.RecordsBaseURL
	if base == "" { base = "https://www.icloud.com/shortcuts/api/records/" }

	recordURL := strings.TrimRight(base, "/") + "/" + id
	recordBytes, err := getLimited(ctx, client, recordURL, 2<<20)
	if err != nil { return nil, fmt.Errorf("iCloud record: %w", err) }
	var record recordResponse
	if err = json.Unmarshal(recordBytes, &record); err != nil { return nil, fmt.Errorf("invalid iCloud record: %w", err) }
	downloadURL := record.Fields.Shortcut.Value.DownloadURL
	if downloadURL == "" { return nil, errors.New("iCloud record did not contain a Shortcut download URL") }
	if err = validatePublicHTTPS(downloadURL); err != nil { return nil, fmt.Errorf("unsafe iCloud asset URL: %w", err) }
	data, err := getLimited(ctx, client, downloadURL, limit)
	if err != nil { return nil, fmt.Errorf("download Shortcut plist: %w", err) }
	if len(data) < 8 { return nil, errors.New("downloaded Shortcut is too small") }
	return &Result{ID: id, Name: record.Fields.Name.Value, Data: data}, nil
}

func getLimited(ctx context.Context, client *http.Client, raw string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil { return nil, err }
	req.Header.Set("User-Agent", "Cherri-Shortcut-Corpus/1")
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		response, requestErr := client.Do(req.Clone(ctx))
		if requestErr != nil {
			last = requestErr
			if attempt < 2 && !sleepContext(ctx, time.Duration(attempt+1)*250*time.Millisecond) { return nil, ctx.Err() }
			continue
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			delay := retryDelay(response, attempt)
			response.Body.Close()
			last = fmt.Errorf("HTTP %d", response.StatusCode)
			if attempt < 2 && !sleepContext(ctx, delay) { return nil, ctx.Err() }
			continue
		}
		if response.StatusCode != http.StatusOK { response.Body.Close(); return nil, fmt.Errorf("HTTP %d", response.StatusCode) }
		reader := io.LimitReader(response.Body, maxBytes+1)
		data, readErr := io.ReadAll(reader); response.Body.Close()
		if readErr != nil { return nil, readErr }
		if int64(len(data)) > maxBytes { return nil, fmt.Errorf("response exceeds %d-byte limit", maxBytes) }
		return data, nil
	}
	return nil, last
}

func retryDelay(response *http.Response, attempt int) time.Duration {
	value := strings.TrimSpace(response.Header.Get("Retry-After"))
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		delay := time.Duration(seconds) * time.Second
		if delay > 30*time.Second { return 30 * time.Second }
		return delay
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay < 0 { return 0 }
		if delay > 30*time.Second { return 30 * time.Second }
		return delay
	}
	return time.Duration(attempt+1) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func secureClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil { return nil, err }
		if strings.EqualFold(host, "localhost") { return nil, errors.New("local host rejected") }
		if literal := net.ParseIP(host); literal != nil {
			if !isPublicIP(literal) { return nil, errors.New("non-public IP rejected") }
			return dialer.DialContext(ctx, network, net.JoinHostPort(literal.String(), port))
		}
		addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil { return nil, err }
		if len(addresses) == 0 { return nil, errors.New("host resolved to no addresses") }
		for _, resolved := range addresses {
			if !isPublicIP(resolved) { return nil, errors.New("host resolved to non-public IP") }
		}
		var last error
		for _, resolved := range addresses {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
			if dialErr == nil { return conn, nil }
			last = dialErr
		}
		return nil, last
	}

	client := &http.Client{Timeout: timeout, Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 { return errors.New("too many redirects") }
		return validatePublicHTTPS(req.URL.String())
	}
	return client
}

func validatePublicHTTPS(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil { return err }
	if parsed.Scheme != "https" || parsed.Hostname() == "" { return errors.New("HTTPS URL required") }
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") { return errors.New("local host rejected") }
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) { return errors.New("non-public IP rejected") }
	return nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsUnspecified())
}
