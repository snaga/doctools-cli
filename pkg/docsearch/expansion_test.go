package docsearch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, model string, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestExpansionService_Success(t *testing.T) {
	mock := &mockLLMClient{
		response: "```json\n[\"有給\", \"年次有給休暇\", \"年休\", \"休暇届\"]\n```",
	}

	cfg := LLMConfig{
		QueryExpansion: true,
		Model:          "gemini-2.5-flash",
	}

	svc := NewExpansionService(cfg, mock)
	res := svc.ExpandQuery(context.Background(), "有給")

	expected := []string{"有給", "年次有給休暇", "年休", "休暇届"}
	if !reflect.DeepEqual(res, expected) {
		t.Errorf("expected %v, got %v", expected, res)
	}
}

func TestExpansionService_Fallbacks(t *testing.T) {
	// 1. Empty query returns empty
	svc := NewExpansionService(LLMConfig{QueryExpansion: true}, &mockLLMClient{})
	if res := svc.ExpandQuery(context.Background(), "   "); len(res) != 0 {
		t.Errorf("expected empty result on empty query, got %v", res)
	}

	// 2. Disabled config returns query itself
	svcDisabled := NewExpansionService(LLMConfig{QueryExpansion: false}, &mockLLMClient{
		response: `["展開された語"]`,
	})
	if res := svcDisabled.ExpandQuery(context.Background(), "有給"); !reflect.DeepEqual(res, []string{"有給"}) {
		t.Errorf("expected fallback on disabled, got %v", res)
	}

	// 3. Nil client returns query itself
	svcNilClient := NewExpansionService(LLMConfig{QueryExpansion: true}, nil)
	if res := svcNilClient.ExpandQuery(context.Background(), "有給"); !reflect.DeepEqual(res, []string{"有給"}) {
		t.Errorf("expected fallback on nil client, got %v", res)
	}

	// 4. Client error returns query itself
	svcErr := NewExpansionService(LLMConfig{QueryExpansion: true}, &mockLLMClient{
		err: errors.New("api error"),
	})
	if res := svcErr.ExpandQuery(context.Background(), "有給"); !reflect.DeepEqual(res, []string{"有給"}) {
		t.Errorf("expected fallback on client error, got %v", res)
	}

	// 5. Invalid JSON response returns query itself
	svcInvalidJSON := NewExpansionService(LLMConfig{QueryExpansion: true}, &mockLLMClient{
		response: "not a valid json",
	})
	if res := svcInvalidJSON.ExpandQuery(context.Background(), "有給"); !reflect.DeepEqual(res, []string{"有給"}) {
		t.Errorf("expected fallback on invalid JSON, got %v", res)
	}
}

func TestExpansionService_ClientInitFromEnv(t *testing.T) {
	origKey := os.Getenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", origKey)

	os.Setenv("GEMINI_API_KEY", "test-api-key")
	svc := NewExpansionService(LLMConfig{QueryExpansion: true, Model: "gemini-2.5-flash"}, nil)
	if svc.client == nil {
		t.Errorf("expected client to be initialized from GEMINI_API_KEY")
	}
}

func TestAPIExpand_HTTPEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	mock := &mockLLMClient{
		response: `["有給", "有給休暇", "年休"]`,
	}
	svc := NewExpansionService(LLMConfig{QueryExpansion: true}, mock)
	srv.SetExpansionService(svc)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /api/expand?q=%E6%9C%89%E7%B5%A6
	resp, err := http.Get(ts.URL + "/api/expand?q=%E6%9C%89%E7%B5%A6")
	if err != nil {
		t.Fatalf("GET /api/expand failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var expandResp ExpandResponse
	if err := json.NewDecoder(resp.Body).Decode(&expandResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if expandResp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", expandResp.Status)
	}
	expected := []string{"有給", "有給休暇", "年休"}
	if !reflect.DeepEqual(expandResp.ExpandedKeywords, expected) {
		t.Errorf("expected keywords %v, got %v", expected, expandResp.ExpandedKeywords)
	}

	// 2. Method Not Allowed (POST)
	postResp, _ := http.Post(ts.URL+"/api/expand", "application/json", nil)
	if postResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on POST /api/expand")
	}

	// 3. Fallback when expansionService is nil
	srv.SetExpansionService(nil)
	respNil, err := http.Get(ts.URL + "/api/expand?q=%E6%9C%89%E7%B5%A6")
	if err != nil || respNil.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/expand with nil svc failed")
	}
	var expandNilResp ExpandResponse
	_ = json.NewDecoder(respNil.Body).Decode(&expandNilResp)
	if !reflect.DeepEqual(expandNilResp.ExpandedKeywords, []string{"有給"}) {
		t.Errorf("expected fallback ['有給'], got %v", expandNilResp.ExpandedKeywords)
	}

	// 4. Empty query on nil svc
	respEmpty, _ := http.Get(ts.URL + "/api/expand?q=")
	var expandEmptyResp ExpandResponse
	_ = json.NewDecoder(respEmpty.Body).Decode(&expandEmptyResp)
	if len(expandEmptyResp.ExpandedKeywords) != 0 {
		t.Errorf("expected empty keywords for empty query")
	}
}
