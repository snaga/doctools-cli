package docsearch

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebUI_RootEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET / should return 200 OK with HTML content
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	body := string(bodyBytes)

	// Verify key elements from index.html
	expectedSnippets := []string{
		"DocSearch",
		"id=\"searchInput\"",
		"id=\"searchBtn\"",
		"id=\"suggestionsList\"",
		"id=\"filterBar\"",
		"id=\"expansionBar\"",
		"id=\"resultsList\"",
		"window.name = \"docsearch_main_tab\";",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(body, snippet) {
			t.Errorf("expected HTML body to contain '%s'", snippet)
		}
	}

	// 2. Non-existent path returns 404
	respNotFound, err := http.Get(ts.URL + "/some/unknown/path")
	if err != nil || respNotFound.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 on unknown path, got %v", respNotFound.StatusCode)
	}

	// 3. POST / returns 405 Method Not Allowed
	respBadMethod, err := http.Post(ts.URL+"/", "text/plain", strings.NewReader("bad"))
	if err != nil || respBadMethod.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on POST /, got %v", respBadMethod.StatusCode)
	}
}
