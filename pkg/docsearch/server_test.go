package docsearch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestServer_LaunchEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /launch without params
	resp, err := http.Get(ts.URL + "/launch")
	if err != nil {
		t.Fatalf("GET /launch failed: %v", err)
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

	// Verify required elements: docsearch_main_tab and window.open
	if !strings.Contains(body, "docsearch_main_tab") {
		t.Errorf("expected HTML body to contain 'docsearch_main_tab'")
	}
	if !strings.Contains(body, "window.open") {
		t.Errorf("expected HTML body to contain 'window.open'")
	}
	if !strings.Contains(body, `const targetUrl = "/";`) {
		t.Errorf("expected HTML body to default targetUrl to '/', got: %s", body)
	}

	// 2. GET /launch with query params (?q=test&indexes=rules)
	respQuery, err := http.Get(ts.URL + "/launch?q=test&indexes=rules")
	if err != nil {
		t.Fatalf("GET /launch with params failed: %v", err)
	}
	defer respQuery.Body.Close()

	if respQuery.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", respQuery.StatusCode)
	}

	queryBodyBytes, err := io.ReadAll(respQuery.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	queryBody := string(queryBodyBytes)

	if !strings.Contains(queryBody, `const targetUrl = "/?q=test&indexes=rules";`) {
		t.Errorf("expected targetUrl to contain query parameters, got: %s", queryBody)
	}

	// 3. POST /launch returns 405 Method Not Allowed
	respBadMethod, err := http.Post(ts.URL+"/launch", "text/plain", strings.NewReader("bad"))
	if err != nil || respBadMethod.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on POST /launch, got %v", respBadMethod.StatusCode)
	}
}

func TestServer_LaunchURL(t *testing.T) {
	srv, _ := setupTestServer(t)

	// 1. Without params
	urlNoParams := srv.LaunchURL("", nil)
	expectedNoParams := fmt.Sprintf("http://127.0.0.1:%d/launch", srv.config.Server.Port)
	if urlNoParams != expectedNoParams {
		t.Errorf("expected %s, got %s", expectedNoParams, urlNoParams)
	}

	// 2. With query only
	urlQuery := srv.LaunchURL("有給 申請", nil)
	expectedQuery := fmt.Sprintf("http://127.0.0.1:%d/launch?q=%%E6%%9C%%89%%E7%%B5%%A6+%%E7%%94%%B3%%E8%%AB%%8B", srv.config.Server.Port)
	if urlQuery != expectedQuery {
		t.Errorf("expected %s, got %s", expectedQuery, urlQuery)
	}

	// 3. With query and indexes
	urlAll := srv.LaunchURL("test", []string{"rules", "project_a"})
	expectedAll := fmt.Sprintf("http://127.0.0.1:%d/launch?q=test&indexes=rules,project_a", srv.config.Server.Port)
	if urlAll != expectedAll {
		t.Errorf("expected %s, got %s", expectedAll, urlAll)
	}

	// 4. With active listener (dynamic port)
	dynamicSrv, _ := NewServer(&Config{Server: ServerConfig{Host: "127.0.0.1", Port: 0}}, "")
	if err := dynamicSrv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("failed to start dynamic server: %v", err)
	}
	defer dynamicSrv.Close()

	dynURL := dynamicSrv.LaunchURL("test", nil)
	if !strings.HasPrefix(dynURL, "http://127.0.0.1:") || !strings.HasSuffix(dynURL, "/launch?q=test") {
		t.Errorf("expected dynamic listener URL, got: %s", dynURL)
	}
	if strings.Contains(dynURL, ":0/") {
		t.Errorf("dynamic listener URL should not contain port 0: %s", dynURL)
	}
}

func TestAPIIndexes_Get(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	// Add an index with a non-existent path to test Exists=false
	missingPath := filepath.Join(t.TempDir(), "non_existent.bleve")
	srv.mu.Lock()
	srv.config.Indexes = append(srv.config.Indexes, IndexConfig{
		ID:              "missing_idx",
		Name:            "見つからないインデックス",
		Path:            missingPath,
		DefaultSelected: false,
	})
	srv.mu.Unlock()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/indexes")
	if err != nil {
		t.Fatalf("failed to GET /api/indexes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var items []IndexItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 index items, got %d", len(items))
	}

	foundMap := make(map[string]IndexItemResponse)
	for _, it := range items {
		foundMap[it.ID] = it
	}

	rulesItem, ok := foundMap["rules"]
	if !ok {
		t.Errorf("rules index not returned")
	} else {
		if !rulesItem.Exists {
			t.Errorf("expected rules index Exists=true")
		}
		if !rulesItem.DefaultSelected {
			t.Errorf("expected rules index DefaultSelected=true")
		}
	}

	missingItem, ok := foundMap["missing_idx"]
	if !ok {
		t.Errorf("missing_idx not returned")
	} else {
		if missingItem.Exists {
			t.Errorf("expected missing_idx Exists=false")
		}
		if missingItem.DefaultSelected {
			t.Errorf("expected missing_idx DefaultSelected=false")
		}
	}
}

func TestAPIIndexes_Post(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	srv.SetConfigFile(configFile)

	// Create a valid directory for a new Bleve index
	newIdxDir := filepath.Join(tmpDir, "my_new_index.bleve")
	if err := os.MkdirAll(newIdxDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Success case: add new index with auto-derived name/ID
	reqBody := map[string]interface{}{
		"path":             newIdxDir,
		"default_selected": true,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	resp, err := http.Post(ts.URL+"/api/indexes", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST /api/indexes failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 201 Created, got %d: %s", resp.StatusCode, string(respBody))
	}

	var created IndexItemResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if created.ID != "my_new_index" {
		t.Errorf("expected auto-derived ID 'my_new_index', got '%s'", created.ID)
	}
	if !created.Exists {
		t.Errorf("expected Exists=true")
	}

	// Verify configFile was persisted
	savedCfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	foundInConfig := false
	for _, idx := range savedCfg.Indexes {
		if idx.ID == "my_new_index" {
			foundInConfig = true
			break
		}
	}
	if !foundInConfig {
		t.Errorf("expected new index in saved config.json")
	}

	// 2. Duplicate error case: same ID or Path
	dupResp, err := http.Post(ts.URL+"/api/indexes", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("POST duplicate failed: %v", err)
	}
	defer dupResp.Body.Close()
	if dupResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on duplicate index, got %d", dupResp.StatusCode)
	}

	// 3. Validation error: empty path
	badReqBody, _ := json.Marshal(map[string]interface{}{"path": ""})
	badResp, err := http.Post(ts.URL+"/api/indexes", "application/json", bytes.NewReader(badReqBody))
	if err != nil {
		t.Fatalf("POST bad request failed: %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty path, got %d", badResp.StatusCode)
	}

	// 4. Validation error: non-existent path
	missingReqBody, _ := json.Marshal(map[string]interface{}{"path": filepath.Join(tmpDir, "not_exist")})
	missingResp, err := http.Post(ts.URL+"/api/indexes", "application/json", bytes.NewReader(missingReqBody))
	if err != nil {
		t.Fatalf("POST non-existent path failed: %v", err)
	}
	defer missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on non-existent path, got %d", missingResp.StatusCode)
	}
}

func TestAPIIndexes_Delete(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	srv.SetConfigFile(configFile)

	// Pre-open rules index in pool by executing a search
	_, err := srv.MultiIndexSearch("有給", []string{"rules"}, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Verify rules index is cached in pool
	srv.mu.RLock()
	_, inPool := srv.indexPool["rules"]
	srv.mu.RUnlock()
	if !inPool {
		t.Fatalf("rules index expected to be in pool")
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Delete via query param ?id=rules
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/indexes?id=rules", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/indexes?id=rules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
	}

	// Verify index is removed from server.config and pool
	srv.mu.RLock()
	_, stillInPool := srv.indexPool["rules"]
	hasRulesConfig := false
	for _, idx := range srv.config.Indexes {
		if idx.ID == "rules" {
			hasRulesConfig = true
		}
	}
	srv.mu.RUnlock()

	if stillInPool {
		t.Errorf("rules index should be removed from indexPool")
	}
	if hasRulesConfig {
		t.Errorf("rules index should be removed from config.Indexes")
	}

	// Verify config was saved to file
	savedCfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	for _, idx := range savedCfg.Indexes {
		if idx.ID == "rules" {
			t.Errorf("deleted index should not exist in saved config")
		}
	}

	// 2. Delete via JSON body {"id": "project_a"}
	delBody, _ := json.Marshal(map[string]string{"id": "project_a"})
	reqBody, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/indexes", bytes.NewReader(delBody))
	reqBody.Header.Set("Content-Type", "application/json")
	respBody, err := http.DefaultClient.Do(reqBody)
	if err != nil {
		t.Fatalf("DELETE with body failed: %v", err)
	}
	defer respBody.Body.Close()

	if respBody.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", respBody.StatusCode)
	}

	// 3. Delete non-existent index -> 404 Not Found
	reqNotFound, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/indexes?id=unknown_idx", nil)
	respNotFound, err := http.DefaultClient.Do(reqNotFound)
	if err != nil {
		t.Fatalf("DELETE unknown failed: %v", err)
	}
	defer respNotFound.Body.Close()
	if respNotFound.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", respNotFound.StatusCode)
	}

	// 4. Delete with missing ID -> 400 Bad Request
	reqMissing, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/indexes", bytes.NewReader([]byte("{}")))
	reqMissing.Header.Set("Content-Type", "application/json")
	respMissing, err := http.DefaultClient.Do(reqMissing)
	if err != nil {
		t.Fatalf("DELETE missing ID failed: %v", err)
	}
	defer respMissing.Body.Close()
	if respMissing.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", respMissing.StatusCode)
	}
}

func TestAPIIndexes_Put(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")
	srv.SetConfigFile(configFile)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Update selection: rules -> false, project_a -> false
	putBody, _ := json.Marshal([]map[string]interface{}{
		{"id": "rules", "selected": false},
		{"id": "project_a", "default_selected": false},
	})

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/indexes", bytes.NewReader(putBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/indexes failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	// Verify in memory
	srv.mu.RLock()
	for _, idx := range srv.config.Indexes {
		if idx.ID == "rules" && idx.DefaultSelected {
			t.Errorf("expected rules DefaultSelected=false")
		}
		if idx.ID == "project_a" && idx.DefaultSelected {
			t.Errorf("expected project_a DefaultSelected=false")
		}
	}
	srv.mu.RUnlock()

	// Verify persisted config
	savedCfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	for _, idx := range savedCfg.Indexes {
		if idx.ID == "rules" && idx.DefaultSelected {
			t.Errorf("persisted config: expected rules DefaultSelected=false")
		}
		if idx.ID == "project_a" && idx.DefaultSelected {
			t.Errorf("persisted config: expected project_a DefaultSelected=false")
		}
	}

	// Bad JSON -> 400
	reqBad, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/indexes", strings.NewReader("invalid-json"))
	reqBad.Header.Set("Content-Type", "application/json")
	respBad, err := http.DefaultClient.Do(reqBad)
	if err != nil {
		t.Fatalf("PUT bad json failed: %v", err)
	}
	defer respBad.Body.Close()
	if respBad.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", respBad.StatusCode)
	}
}

func TestServer_ConfigFileAndStop(t *testing.T) {
	srv, err := NewServer(nil, "")
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	srv.SetConfigFile("test_config.json")
	if srv.ConfigFile() != "test_config.json" {
		t.Errorf("expected ConfigFile 'test_config.json', got %q", srv.ConfigFile())
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestServer_SSE(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	if srv.HasActiveWebClients() {
		t.Fatalf("expected HasActiveWebClients to be false initially")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// Verify HasActiveWebClients is true
	var active bool
	for i := 0; i < 50; i++ {
		if srv.HasActiveWebClients() {
			active = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !active {
		t.Fatalf("expected HasActiveWebClients to be true")
	}

	reader := bufio.NewReader(resp.Body)

	// Read initial connect event
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read initial event: %v", err)
	}
	if !strings.Contains(line, "connect") && !strings.Contains(line, "ok") {
		line2, _ := reader.ReadString('\n')
		if !strings.Contains(line+line2, "connect") && !strings.Contains(line+line2, "ok") {
			t.Errorf("unexpected initial line: %q + %q", line, line2)
		}
	}

	// Broadcast focus notification
	srv.NotifyWebClients("focus")

	// Read from SSE response until "focus" is found or timeout
	foundFocus := false
	msgChan := make(chan string, 1)
	go func() {
		for {
			l, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(l, `"action":"focus"`) || strings.Contains(l, `"action": "focus"`) {
				msgChan <- l
				return
			}
		}
	}()

	select {
	case <-msgChan:
		foundFocus = true
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for 'focus' action in SSE stream")
	}
	if !foundFocus {
		t.Errorf("expected focus message from SSE stream")
	}

	// Cancel context to simulate client disconnect
	cancel()
	_ = resp.Body.Close()

	// Verify HasActiveWebClients becomes false
	var inactive bool
	for i := 0; i < 50; i++ {
		if !srv.HasActiveWebClients() {
			inactive = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !inactive {
		t.Errorf("expected HasActiveWebClients to become false after context cancellation")
	}
}


