package docsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// LLMClient abstracts the GenAI generation call for unit testability.
type LLMClient interface {
	GenerateContent(ctx context.Context, model string, prompt string) (string, error)
}

// genaiSDKClient wraps google.golang.org/genai client to satisfy LLMClient.
type genaiSDKClient struct {
	client *genai.Client
}

func (c *genaiSDKClient) GenerateContent(ctx context.Context, model string, prompt string) (string, error) {
	result, err := c.client.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return "", err
	}
	return result.Text(), nil
}

// ExpansionService provides LLM-based query expansion and keyword generation.
type ExpansionService struct {
	config LLMConfig
	client LLMClient
}

// NewExpansionService creates an ExpansionService.
// If client is nil and QueryExpansion is enabled, it attempts to initialize a real GenAI client
// using GEMINI_API_KEY from environment variables.
func NewExpansionService(cfg LLMConfig, client LLMClient) *ExpansionService {
	if client == nil && cfg.QueryExpansion {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey != "" {
			ctx := context.Background()
			c, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey: apiKey,
			})
			if err == nil {
				client = &genaiSDKClient{client: c}
			}
		}
	}

	return &ExpansionService{
		config: cfg,
		client: client,
	}
}

// ExpandQuery expands a search query with synonyms, abbreviations, and related terms.
// If expansion is disabled, no client is available, or an error occurs, it safely falls back
// to returning a slice containing just the original query.
func (s *ExpansionService) ExpandQuery(ctx context.Context, query string) []string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []string{}
	}

	// Graceful fallback if disabled or no LLM client configured
	if !s.config.QueryExpansion || s.client == nil {
		return []string{trimmed}
	}

	model := s.config.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}

	prompt := fmt.Sprintf(`あなたは日本語の社内文書や技術文書の検索エキスパートです。
以下の検索キーワード「%s」について、検索漏れを防ぐための同義語、類義語、略称、正式名称、表記揺れを生成してください。
必ず入力キーワードそのものを含めた、JSONの文字列配列形式（例: ["有給", "年次有給休暇", "年休", "有給休暇"]）のみを出力してください。余計な説明文やマークダウン記法は不要です。`, trimmed)

	respText, err := s.client.GenerateContent(ctx, model, prompt)
	if err != nil {
		return []string{trimmed}
	}

	// Clean up markdown fences if returned
	cleaned := strings.TrimSpace(respText)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
	}
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var keywords []string
	if err := json.Unmarshal([]byte(cleaned), &keywords); err != nil {
		return []string{trimmed}
	}

	// De-duplicate keywords and preserve trimmed query as first element
	seen := make(map[string]bool)
	unique := make([]string, 0, len(keywords)+1)

	seen[trimmed] = true
	unique = append(unique, trimmed)

	for _, kw := range keywords {
		kwTrim := strings.TrimSpace(kw)
		if kwTrim != "" && !seen[kwTrim] {
			seen[kwTrim] = true
			unique = append(unique, kwTrim)
		}
	}

	return unique
}
