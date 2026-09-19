package docsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// SearchResultItem represents a single document hit in multi-index search.
type SearchResultItem struct {
	IndexID   string    `json:"index_id"`
	IndexName string    `json:"index_name"`
	Score     float64   `json:"score"`
	FileName  string    `json:"file_name"`
	FilePath  string    `json:"file_path"`
	FileType  string    `json:"file_type"`
	Page      int       `json:"page"`
	Snippet   string    `json:"snippet"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResponse represents the REST API response for multi-index search.
type SearchResponse struct {
	Status           string             `json:"status"`
	Query            string             `json:"query"`
	TotalHits        int                `json:"total_hits"`
	IndexCounts      map[string]int     `json:"index_counts"`
	ExpandedKeywords []string           `json:"expanded_keywords"`
	Results          []SearchResultItem `json:"results"`
}

// SuggestResponse represents the REST API response for query suggestions.
type SuggestResponse struct {
	Status      string   `json:"status"`
	Prefix      string   `json:"prefix"`
	Suggestions []string `json:"suggestions"`
}

// ExpandResponse represents the response for query expansion.
type ExpandResponse struct {
	Status           string   `json:"status"`
	Query            string   `json:"query"`
	ExpandedKeywords []string `json:"expanded_keywords"`
}

// OpenRequest represents the request body for opening a document.
type OpenRequest struct {
	Path   string `json:"path"`
	Select bool   `json:"select"` // If true, select in file manager
}

// OpenResponse represents the result of opening a document.
type OpenResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// execCommandFn is a variable for command execution to allow testing.
var execCommandFn = exec.Command

// Server manages the local HTTP REST server and Bleve search indexes.
type Server struct {
	config           *Config
	historyPath      string
	expansionService *ExpansionService
	mu               sync.RWMutex
	wg               sync.WaitGroup
	indexPool        map[string]bleve.Index // indexID -> bleve.Index
	httpServer       *http.Server
	listener         net.Listener
}

// NewServer creates a new Server instance with the specified configuration and history path.
func NewServer(cfg *Config, historyPath string) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Server{
		config:           cfg,
		historyPath:      historyPath,
		expansionService: NewExpansionService(cfg.LLM, nil),
		indexPool:        make(map[string]bleve.Index),
	}, nil
}

// SetExpansionService sets the ExpansionService instance (useful for testing and dependency injection).
func (s *Server) SetExpansionService(svc *ExpansionService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expansionService = svc
}

// getOrOpenIndex opens or returns a cached Bleve index for the specified index config.
func (s *Server) getOrOpenIndex(idxCfg IndexConfig) (bleve.Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if idx, ok := s.indexPool[idxCfg.ID]; ok {
		return idx, nil
	}

	idx, err := bleve.Open(idxCfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bleve index %s (%s): %w", idxCfg.ID, idxCfg.Path, err)
	}

	s.indexPool[idxCfg.ID] = idx
	return idx, nil
}

// MultiIndexSearch executes full-text search across the specified Bleve indexes.
func (s *Server) MultiIndexSearch(query string, indexIDs []string, limit int) (*SearchResponse, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return &SearchResponse{
			Status:           "success",
			Query:            "",
			TotalHits:        0,
			IndexCounts:      make(map[string]int),
			ExpandedKeywords: make([]string, 0),
			Results:          make([]SearchResultItem, 0),
		}, nil
	}

	if limit <= 0 {
		limit = 20
	}

	// Record search query in history asynchronously
	if s.historyPath != "" {
		s.wg.Add(1)
		go func(p, q string) {
			defer s.wg.Done()
			_ = AddHistory(p, q)
		}(s.historyPath, trimmedQuery)
	}

	// Determine target indexes
	selectedIDMap := make(map[string]bool)
	for _, id := range indexIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			selectedIDMap[trimmed] = true
		}
	}

	var targetConfigs []IndexConfig
	for _, idx := range s.config.Indexes {
		if len(selectedIDMap) == 0 {
			if idx.DefaultSelected {
				targetConfigs = append(targetConfigs, idx)
			}
		} else if selectedIDMap[idx.ID] {
			targetConfigs = append(targetConfigs, idx)
		}
	}

	if len(targetConfigs) == 0 && len(s.config.Indexes) > 0 {
		// Fallback to all indexes if none selected
		targetConfigs = s.config.Indexes
	}

	if len(targetConfigs) == 0 {
		return &SearchResponse{
			Status:           "success",
			Query:            trimmedQuery,
			TotalHits:        0,
			IndexCounts:      make(map[string]int),
			ExpandedKeywords: make([]string, 0),
			Results:          make([]SearchResultItem, 0),
		}, nil
	}

	indexCounts := make(map[string]int)
	alias := bleve.NewIndexAlias()
	indexConfigByID := make(map[string]IndexConfig)
	openedCount := 0

	for _, cfg := range targetConfigs {
		indexCounts[cfg.ID] = 0
		indexConfigByID[cfg.ID] = cfg

		idx, err := s.getOrOpenIndex(cfg)
		if err != nil {
			// Skip inaccessible indexes gracefully
			continue
		}
		openedCount++

		// Count hits for this index
		countReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(trimmedQuery), 0, 0, false)
		countRes, countErr := idx.Search(countReq)
		if countErr == nil && countRes != nil {
			indexCounts[cfg.ID] = int(countRes.Total)
		}

		alias.Add(idx)
	}

	if openedCount == 0 {
		return &SearchResponse{
			Status:           "success",
			Query:            trimmedQuery,
			TotalHits:        0,
			IndexCounts:      indexCounts,
			ExpandedKeywords: make([]string, 0),
			Results:          make([]SearchResultItem, 0),
		}, nil
	}

	// Execute unified cross-index search
	searchReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(trimmedQuery), limit, 0, false)
	searchReq.Highlight = bleve.NewHighlight()
	searchReq.Fields = []string{"*"}

	searchRes, err := alias.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search bleve index alias: %w", err)
	}

	results := make([]SearchResultItem, 0, len(searchRes.Hits))
	for _, match := range searchRes.Hits {
		var snippet string
		if frags, ok := match.Fragments["content"]; ok && len(frags) > 0 {
			snippet = strings.Join(frags, " ... ")
		} else if contentVal, ok := match.Fields["content"].(string); ok {
			runes := []rune(contentVal)
			if len(runes) > 200 {
				snippet = string(runes[:200]) + "..."
			} else {
				snippet = contentVal
			}
		}

		filePath, _ := match.Fields["file_path"].(string)
		fileName, _ := match.Fields["file_name"].(string)
		fileType, _ := match.Fields["file_type"].(string)

		var page int
		if poiVal, ok := match.Fields["page_or_index"].(float64); ok {
			page = int(poiVal)
		} else if poiValInt, ok := match.Fields["page_or_index"].(int); ok {
			page = poiValInt
		}

		var updatedAt time.Time
		if tVal, ok := match.Fields["updated_at"].(string); ok {
			if t, parseErr := time.Parse(time.RFC3339, tVal); parseErr == nil {
				updatedAt = t
			}
		} else if t, ok := match.Fields["updated_at"].(time.Time); ok {
			updatedAt = t
		}

		// Determine associated index ID
		assignedID := ""
		assignedName := ""
		for _, cfg := range targetConfigs {
			if assignedID == "" {
				assignedID = cfg.ID
				assignedName = cfg.Name
			}
			// If file path starts with index root or match ID contains it
			idxDir := filepath.Dir(cfg.Path)
			if strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(idxDir)) {
				assignedID = cfg.ID
				assignedName = cfg.Name
				break
			}
		}

		results = append(results, SearchResultItem{
			IndexID:   assignedID,
			IndexName: assignedName,
			Score:     match.Score,
			FileName:  fileName,
			FilePath:  filePath,
			FileType:  strings.TrimPrefix(fileType, "."),
			Page:      page,
			Snippet:   snippet,
			UpdatedAt: updatedAt,
		})
	}

	var expandedKeywords []string
	s.mu.RLock()
	expSvc := s.expansionService
	s.mu.RUnlock()
	if expSvc != nil {
		expandedKeywords = expSvc.ExpandQuery(context.Background(), trimmedQuery)
	} else {
		expandedKeywords = make([]string, 0)
	}

	return &SearchResponse{
		Status:           "success",
		Query:            trimmedQuery,
		TotalHits:        int(searchRes.Total),
		IndexCounts:      indexCounts,
		ExpandedKeywords: expandedKeywords,
		Results:          results,
	}, nil
}

// OpenDocument opens the file or directory using the OS default application.
func OpenDocument(path string, selectInFolder bool) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Verify file or directory existence
	if _, err := os.Stat(trimmed); err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if runtime.GOOS == "windows" {
		if selectInFolder {
			cmd := execCommandFn("explorer.exe", fmt.Sprintf("/select,%s", trimmed))
			return cmd.Start()
		}
		cmd := execCommandFn("rundll32.exe", "url.dll,FileProtocolHandler", trimmed)
		return cmd.Start()
	}

	// Non-Windows fallbacks
	if runtime.GOOS == "darwin" {
		cmd := execCommandFn("open", trimmed)
		return cmd.Start()
	}

	cmd := execCommandFn("xdg-open", trimmed)
	return cmd.Start()
}

// Handler returns the HTTP Handler for docsearch REST endpoints.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/history/suggest", s.handleSuggest)
	mux.HandleFunc("/api/expand", s.handleExpand)
	mux.HandleFunc("/api/open", s.handleOpen)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	indexesParam := r.URL.Query().Get("indexes")
	limitParam := r.URL.Query().Get("limit")

	limit := 20
	if limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	var indexIDs []string
	if indexesParam != "" {
		for _, id := range strings.Split(indexesParam, ",") {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" {
				indexIDs = append(indexIDs, trimmed)
			}
		}
	}

	res, err := s.MultiIndexSearch(q, indexIDs, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prefix := r.URL.Query().Get("prefix")
	limitParam := r.URL.Query().Get("limit")

	limit := 10
	if limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	suggestions, err := GetSuggestions(s.historyPath, prefix, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuggestResponse{
		Status:      "success",
		Prefix:      prefix,
		Suggestions: suggestions,
	})
}

func (s *Server) handleExpand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")

	s.mu.RLock()
	expSvc := s.expansionService
	s.mu.RUnlock()

	var keywords []string
	if expSvc != nil {
		keywords = expSvc.ExpandQuery(r.Context(), q)
	} else {
		trimmed := strings.TrimSpace(q)
		if trimmed != "" {
			keywords = []string{trimmed}
		} else {
			keywords = []string{}
		}
	}

	writeJSON(w, http.StatusOK, ExpandResponse{
		Status:           "success",
		Query:            q,
		ExpandedKeywords: keywords,
	})
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Path) == "" {
		writeJSONError(w, http.StatusBadRequest, "Path parameter is required")
		return
	}

	if err := OpenDocument(req.Path, req.Select); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, OpenResponse{
		Status:  "success",
		Message: fmt.Sprintf("Opened %s", req.Path),
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "error",
		"message": message,
	})
}

// Start binds the listener and starts the HTTP server in a background goroutine.
func (s *Server) Start(addr string) error {
	s.mu.Lock()
	if s.httpServer != nil {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}

	if addr == "" {
		addr = fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = ln

	httpSrv := &http.Server{
		Handler: s.Handler(),
	}
	s.httpServer = httpSrv
	s.mu.Unlock()

	go func() {
		_ = httpSrv.Serve(ln)
	}()

	return nil
}

// Addr returns the network address the server is listening on.
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// Close gracefully closes the HTTP server and releases all opened Bleve index handles.
func (s *Server) Close() error {
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			firstErr = err
		}
		s.httpServer = nil
	}

	for id, idx := range s.indexPool {
		if err := idx.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close bleve index %s: %w", id, err)
		}
	}
	s.indexPool = make(map[string]bleve.Index)

	return firstErr
}
