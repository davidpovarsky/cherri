package icloudshortcut

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)
func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
func testResponse(status int, body string) *http.Response { return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)} }

func TestCanonicalURL(t *testing.T) {
	canonical, id, err := CanonicalURL("https://icloud.com/shortcuts/1234567890abcdef1234567890abcdef?x=1")
	if err != nil { t.Fatal(err) }
	if canonical != "https://www.icloud.com/shortcuts/1234567890abcdef1234567890abcdef" || id != "1234567890abcdef1234567890abcdef" { t.Fatalf("unexpected canonical result: %s %s", canonical, id) }
}

func TestResolverDownloadsUnderlyingPlist(t *testing.T) {
	const id = "1234567890abcdef1234567890abcdef"
	plist := "bplist00fixture"
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/shortcuts/api/records/") { return testResponse(200, `{"fields":{"name":{"value":"Fixture"},"shortcut":{"value":{"downloadURL":"https://cvws.icloud-content.com/asset"}}}}`), nil }
		if req.URL.Host == "cvws.icloud-content.com" { return testResponse(200, plist), nil }
		return testResponse(404, "missing"), nil
	})}
	resolver := &Resolver{Client: client, MaxBytes: 1024, RecordsBaseURL: "https://www.icloud.com/shortcuts/api/records/"}
	result, err := resolver.Resolve(context.Background(), "https://www.icloud.com/shortcuts/"+id)
	if err != nil { t.Fatal(err) }
	if result.ID != id || result.Name != "Fixture" || string(result.Data) != plist { t.Fatalf("unexpected result: %+v", result) }
}

func TestResolverRejectsOversizedAsset(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/shortcuts/api/records/") { return testResponse(200, `{"fields":{"shortcut":{"value":{"downloadURL":"https://cvws.icloud-content.com/asset"}}}}`), nil }
		return testResponse(200, strings.Repeat("x", 40)), nil
	})}
	resolver := &Resolver{Client: client, MaxBytes: 16, RecordsBaseURL: "https://www.icloud.com/shortcuts/api/records/"}
	if _, err := resolver.Resolve(context.Background(), "https://www.icloud.com/shortcuts/1234567890abcdef1234567890abcdef"); err == nil { t.Fatal("expected response-size rejection") }
}
