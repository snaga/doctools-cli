package docsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

func createTestIndex(t *testing.T, dirPath string, indexName string, docs []map[string]interface{}) string {
	indexPath := filepath.Join(dirPath, indexName+".bleve")
	mapping := bleve.NewIndexMapping()
	idx, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("failed to create bleve index: %v", err)
	}
	defer idx.Close()

	for i, doc := range docs {
		id := fmt.Sprintf("doc-%d", i)
		if err := idx.Index(id, doc); err != nil {
			t.Fatalf("failed to index doc: %v", err)
		}
	}
	return indexPath
}

func setupTestServer(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()

	docsRules := []map[string]interface{}{
		{
			"file_name":     "就業規則_2026.pdf",
			"file_path":     filepath.Join(tmpDir, "rules", "就業規則_2026.pdf"),
			"file_type":     "pdf",
			"page_or_index": 15,
			"content":       "第5条 有給 申請手続きについて規定する",
			"updated_at":    time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
		{
			"file_name":     "出張旅費規程.docx",
			"file_path":     filepath.Join(tmpDir, "rules", "出張旅費規程.docx"),
			"file_type":     "docx",
			"page_or_index": 1,
			"content":       "出張 申請と旅費精算について規定する",
			"updated_at":    time.Date(2026, 3, 2, 11, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}

	docsProjectA := []map[string]interface{}{
		{
			"file_name":     "プロジェクト計画.xlsx",
			"file_path":     filepath.Join(tmpDir, "project_a", "プロジェクト計画.xlsx"),
			"file_type":     "xlsx",
			"page_or_index": 2,
			"content":       "有給 取得計画とリソース配分マトリクス",
			"updated_at":    time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}

	rulesPath := createTestIndex(t, tmpDir, "rules", docsRules)
	projectAPath := createTestIndex(t, tmpDir, "project_a", docsProjectA)
	histPath := filepath.Join(tmpDir, "history.json")

	cfg := &Config{
		Server: ServerConfig{Port: 18080, Host: "127.0.0.1"},
		Indexes: []IndexConfig{
			{ID: "rules", Name: "社内規程", Path: rulesPath, DefaultSelected: true},
			{ID: "project_a", Name: "案件A", Path: projectAPath, DefaultSelected: true},
		},
	}

	srv, err := NewServer(cfg, histPath)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	t.Cleanup(func() {
		_ = srv.Close()
	})

	return srv, histPath
}

func TestMultiIndexSearch(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// 1. Cross-index search for "有給"
	res, err := srv.MultiIndexSearch("有給", nil, 10)
	if err != nil {
		t.Fatalf("MultiIndexSearch error: %v", err)
	}

	if res.TotalHits != 2 {
		t.Errorf("expected total hits 2, got %d", res.TotalHits)
	}
	if res.IndexCounts["rules"] != 1 {
		t.Errorf("expected rules count 1, got %d", res.IndexCounts["rules"])
	}
	if res.IndexCounts["project_a"] != 1 {
		t.Errorf("expected project_a count 1, got %d", res.IndexCounts["project_a"])
	}
	if len(res.Results) != 2 {
		t.Fatalf("expected 2 result items, got %d", len(res.Results))
	}

	// 2. Single index search for "有給" filtered by "rules"
	resRules, err := srv.MultiIndexSearch("有給", []string{"rules"}, 10)
	if err != nil {
		t.Fatalf("MultiIndexSearch error: %v", err)
	}
	if resRules.TotalHits != 1 {
		t.Errorf("expected total hits 1, got %d", resRules.TotalHits)
	}

	// 3. Search query "出張" (only in rules)
	resTrip, err := srv.MultiIndexSearch("出張", nil, 10)
	if err != nil {
		t.Fatalf("MultiIndexSearch error: %v", err)
	}
	if resTrip.TotalHits != 1 {
		t.Errorf("expected total hits 1, got %d", resTrip.TotalHits)
	}

	// 4. Empty query returns empty response
	emptyRes, err := srv.MultiIndexSearch("   ", nil, 10)
	if err != nil {
		t.Fatalf("unexpected error on empty query: %v", err)
	}
	if emptyRes.TotalHits != 0 || len(emptyRes.Results) != 0 {
		t.Errorf("expected empty search result, got %v", emptyRes)
	}
}

func TestAPISearch_HTTPEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// GET /api/search?q=有給&limit=5
	resp, err := http.Get(ts.URL + "/api/search?q=有給&limit=5")
	if err != nil {
		t.Fatalf("GET /api/search failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if searchResp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", searchResp.Status)
	}
	if searchResp.TotalHits != 2 {
		t.Errorf("expected 2 hits, got %d", searchResp.TotalHits)
	}

	// GET /api/search?q=%E6%9C%89%E7%B5%A6 (omitted indexes and limit)
	respDefault, err := http.Get(ts.URL + "/api/search?q=%E6%9C%89%E7%B5%A6")
	if err != nil || respDefault.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on default indexes search")
	}
	_ = respDefault.Body.Close()

	// GET with comma separated indexes
	respFiltered, err := http.Get(ts.URL + "/api/search?q=%E6%9C%89%E7%B5%A6&indexes=rules,project_a")
	if err != nil || respFiltered.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on filtered search")
	}
	_ = respFiltered.Body.Close()

	// OPTIONS preflight check
	req, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/search", nil)
	optionsResp, err := http.DefaultClient.Do(req)
	if err != nil || optionsResp.StatusCode != http.StatusOK {
		t.Errorf("expected OPTIONS 200 OK")
	}
	if optionsResp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing CORS header")
	}

	// Invalid method POST on /api/search
	badMethodResp, _ := http.Post(ts.URL+"/api/search", "application/json", nil)
	if badMethodResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed")
	}
}

func TestAPIHistorySuggest_HTTPEndpoint(t *testing.T) {
	srv, histPath := setupTestServer(t)
	defer srv.Close()

	_ = AddHistory(histPath, "有給 申請")
	_ = AddHistory(histPath, "有給 残日数")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// GET /api/history/suggest?prefix=有給&limit=10
	resp, err := http.Get(ts.URL + "/api/history/suggest?prefix=有給&limit=10")
	if err != nil {
		t.Fatalf("GET /api/history/suggest failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var suggestResp SuggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&suggestResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if suggestResp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", suggestResp.Status)
	}
	if len(suggestResp.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d (%v)", len(suggestResp.Suggestions), suggestResp.Suggestions)
	}
}

func TestAPIOpen_HTTPEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Mock execCommandFn to avoid launching real applications
	origExec := execCommandFn
	defer func() { execCommandFn = origExec }()

	var executedCmd string
	execCommandFn = func(name string, arg ...string) *exec.Cmd {
		executedCmd = name
		// Return a harmless echo or exit command
		if name == "explorer.exe" || name == "rundll32.exe" || name == "open" || name == "xdg-open" {
			return exec.Command("cmd.exe", "/c", "echo ok")
		}
		return exec.Command("echo", "ok")
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Create a real temp file
	tmpFile := filepath.Join(t.TempDir(), "dummy.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// 1. Success open
	payload, _ := json.Marshal(OpenRequest{Path: tmpFile, Select: false})
	resp, err := http.Post(ts.URL+"/api/open", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/open failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if executedCmd == "" {
		t.Errorf("expected command execution")
	}

	// 2. Select in folder
	payloadSelect, _ := json.Marshal(OpenRequest{Path: tmpFile, Select: true})
	respSelect, _ := http.Post(ts.URL+"/api/open", "application/json", bytes.NewReader(payloadSelect))
	if respSelect.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 on select, got %d", respSelect.StatusCode)
	}

	// 3. Non-existent file
	badPayload, _ := json.Marshal(OpenRequest{Path: filepath.Join(t.TempDir(), "nonexistent.txt")})
	badResp, _ := http.Post(ts.URL+"/api/open", "application/json", bytes.NewReader(badPayload))
	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for non-existent file, got %d", badResp.StatusCode)
	}

	// 4. Empty path
	emptyPayload, _ := json.Marshal(OpenRequest{Path: ""})
	emptyResp, _ := http.Post(ts.URL+"/api/open", "application/json", bytes.NewReader(emptyPayload))
	if emptyResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty path, got %d", emptyResp.StatusCode)
	}

	// 5. Invalid JSON body
	invalidJSONResp, _ := http.Post(ts.URL+"/api/open", "application/json", bytes.NewReader([]byte("not-json")))
	if invalidJSONResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", invalidJSONResp.StatusCode)
	}
}

func TestServer_Lifecycle(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Start server on dynamic port
	err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	addr := srv.Addr()
	if addr == "" {
		t.Errorf("expected non-empty addr")
	}

	// Stop server
	if err := srv.Close(); err != nil {
		t.Fatalf("failed to close server: %v", err)
	}

	// Close again should be safe
	if err := srv.Close(); err != nil {
		t.Errorf("second close should not error")
	}

	// Start with empty addr (defaults to config Host:Port)
	portSrv, _ := NewServer(&Config{Server: ServerConfig{Host: "127.0.0.1", Port: 0}}, "")
	if err := portSrv.Start(""); err != nil {
		t.Errorf("expected Start with empty addr to succeed with port 0: %v", err)
	} else {
		_ = portSrv.Close()
	}
}

func TestServer_EdgeCases(t *testing.T) {
	// NewServer with nil config
	nilSrv, err := NewServer(nil, "")
	if err != nil || nilSrv.config == nil {
		t.Errorf("expected default config on nil")
	}

	// Addr when not started
	if addr := nilSrv.Addr(); addr != "" {
		t.Errorf("expected empty addr when not listening, got %s", addr)
	}

	// Double start error
	if err := nilSrv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("failed first start: %v", err)
	}
	defer nilSrv.Close()

	if err := nilSrv.Start("127.0.0.1:0"); err == nil {
		t.Errorf("expected error on duplicate start")
	}

	// Method not allowed on suggest
	ts := httptest.NewServer(nilSrv.Handler())
	defer ts.Close()

	badSuggest, _ := http.Post(ts.URL+"/api/history/suggest", "application/json", nil)
	if badSuggest.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on POST suggest")
	}

	// Search with non-existent index ID in filter
	badIdxResp, err := nilSrv.MultiIndexSearch("test", []string{"non_existent_idx"}, 5)
	if err != nil || badIdxResp.TotalHits != 0 {
		t.Errorf("expected 0 hits on non-existent index")
	}

	// OpenDocument with empty path
	if err := OpenDocument("", false); err == nil {
		t.Errorf("expected error opening empty path")
	}

	// Invalid limit parameters
	resp, _ := http.Get(ts.URL + "/api/search?q=test&limit=invalid")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on invalid limit")
	}

	respSuggest, _ := http.Get(ts.URL + "/api/history/suggest?prefix=test&limit=invalid")
	if respSuggest.StatusCode != http.StatusOK {
		t.Errorf("expected 200 on invalid suggest limit")
	}

	// Method not allowed on open
	respBadOpen, _ := http.Get(ts.URL + "/api/open")
	if respBadOpen.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on GET /api/open")
	}

	// OpenDocument direct calls
	tmpFile := filepath.Join(t.TempDir(), "direct_test.txt")
	_ = os.WriteFile(tmpFile, []byte("ok"), 0644)
	if err := OpenDocument(tmpFile, false); err != nil {
		t.Errorf("expected direct OpenDocument to succeed")
	}
	if err := OpenDocument(tmpFile, true); err != nil {
		t.Errorf("expected direct OpenDocument with select to succeed")
	}
	if err := OpenDocument(filepath.Join(t.TempDir(), "nonexistent"), false); err == nil {
		t.Errorf("expected direct OpenDocument with non-existent path to fail")
	}

	// getOrOpenIndex cache hit test on setupTestServer
	testSrv, _ := setupTestServer(t)
	idx1, err1 := testSrv.getOrOpenIndex(testSrv.config.Indexes[0])
	idx2, err2 := testSrv.getOrOpenIndex(testSrv.config.Indexes[0])
	if err1 != nil || err2 != nil || idx1 != idx2 {
		t.Errorf("expected cached index hit")
	}
	if _, err := testSrv.getOrOpenIndex(IndexConfig{ID: "bad", Path: "nonexistent"}); err == nil {
		t.Errorf("expected error opening non-existent index")
	}
}
