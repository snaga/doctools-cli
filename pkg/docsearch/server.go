package docsearch

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

//go:embed web/*
var webFS embed.FS

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

// IndexItemResponse represents the status and configuration of a search index.
type IndexItemResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	DefaultSelected bool   `json:"default_selected"`
	Exists          bool   `json:"exists"`
}

// AddIndexRequest represents the payload for registering a new index.
type AddIndexRequest struct {
	ID              string `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
	Path            string `json:"path"`
	DefaultSelected *bool  `json:"default_selected,omitempty"`
}

// UpdateIndexSelectionItem represents a single index selection update.
type UpdateIndexSelectionItem struct {
	ID              string `json:"id"`
	Selected        *bool  `json:"selected,omitempty"`
	DefaultSelected *bool  `json:"default_selected,omitempty"`
}

// execCommandFn is a variable for command execution to allow testing.
var execCommandFn = exec.Command

// Server manages the local HTTP REST server and Bleve search indexes.
type Server struct {
	config           *Config
	configPath       string
	historyPath      string
	expansionService *ExpansionService
	mu               sync.RWMutex
	wg               sync.WaitGroup
	indexPool        map[string]bleve.Index // indexID -> bleve.Index
	httpServer       *http.Server
	listener         net.Listener

	eventsMu     sync.RWMutex
	eventClients map[chan string]bool
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
		eventClients:     make(map[chan string]bool),
	}, nil
}

// SetConfigFile sets the configuration file path for auto-saving settings.
func (s *Server) SetConfigFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configPath = path
}

// ConfigFile returns the current configuration file path.
func (s *Server) ConfigFile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configPath
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

	bleveQueryStr := TransformJapaneseQuery(trimmedQuery)

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
		countReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(bleveQueryStr), 0, 0, false)
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
	searchReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(bleveQueryStr), limit, 0, false)
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

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/launch", s.handleLaunch)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/history/suggest", s.handleSuggest)
	mux.HandleFunc("/api/expand", s.handleExpand)
	mux.HandleFunc("/api/open", s.handleOpen)
	mux.HandleFunc("/api/indexes", s.handleIndexes)
	mux.HandleFunc("/api/events", s.handleEvents)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "WebUI asset not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := "/"
	if r.URL.RawQuery != "" {
		targetURL = "/?" + r.URL.RawQuery
	}

	targetURLJSON, err := json.Marshal(targetURL)
	targetURLStr := string(targetURLJSON)
	if err != nil {
		targetURLStr = `"/"`
	} else {
		targetURLStr = strings.ReplaceAll(targetURLStr, `\u0026`, "&")
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>DocSearch Launcher</title></head>
<body>
<script>
  const targetName = "docsearch_main_tab";
  const targetUrl = %s;
  const w = window.open(targetUrl, targetName);
  if (w) {
    try { w.focus(); } catch(e) {}
  }
  window.close();
  setTimeout(function() {
    window.location.replace(targetUrl);
  }, 300);
</script>
</body>
</html>
`, targetURLStr)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
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

func (s *Server) handleIndexes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetIndexes(w, r)
	case http.MethodPost:
		s.handlePostIndex(w, r)
	case http.MethodDelete:
		s.handleDeleteIndex(w, r)
	case http.MethodPut:
		s.handlePutIndexes(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetIndexes(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]IndexItemResponse, 0, len(s.config.Indexes))
	for _, idx := range s.config.Indexes {
		exists := false
		if fi, err := os.Stat(idx.Path); err == nil && fi.IsDir() {
			exists = true
		}
		items = append(items, IndexItemResponse{
			ID:              idx.ID,
			Name:            idx.Name,
			Path:            idx.Path,
			DefaultSelected: idx.DefaultSelected,
			Exists:          exists,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePostIndex(w http.ResponseWriter, r *http.Request) {
	var req AddIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	trimmedPath := strings.TrimSpace(req.Path)
	if trimmedPath == "" {
		writeJSONError(w, http.StatusBadRequest, "Path parameter is required")
		return
	}

	generated, err := GenerateIndexConfigFromPath(trimmedPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = generated.ID
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = generated.Name
	}
	defaultSelected := true
	if req.DefaultSelected != nil {
		defaultSelected = *req.DefaultSelected
	}

	cleanPath := filepath.Clean(trimmedPath)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.config.Indexes {
		if existing.ID == id {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("index with id '%s' already exists", id))
			return
		}
		if filepath.Clean(existing.Path) == cleanPath {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("index with path '%s' already exists", cleanPath))
			return
		}
	}

	newIdx := IndexConfig{
		ID:              id,
		Name:            name,
		Path:            cleanPath,
		DefaultSelected: defaultSelected,
	}
	s.config.Indexes = append(s.config.Indexes, newIdx)

	if s.configPath != "" {
		if err := SaveConfig(s.configPath, s.config); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, IndexItemResponse{
		ID:              newIdx.ID,
		Name:            newIdx.Name,
		Path:            newIdx.Path,
		DefaultSelected: newIdx.DefaultSelected,
		Exists:          true,
	})
}

func (s *Server) handleDeleteIndex(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" && r.Body != nil {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = strings.TrimSpace(req.ID)
		}
	}

	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Index ID is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	newIndexes := make([]IndexConfig, 0, len(s.config.Indexes))
	for _, idx := range s.config.Indexes {
		if idx.ID == id {
			found = true
		} else {
			newIndexes = append(newIndexes, idx)
		}
	}

	if !found {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("index '%s' not found", id))
		return
	}

	s.config.Indexes = newIndexes

	if openedIdx, ok := s.indexPool[id]; ok {
		_ = openedIdx.Close()
		delete(s.indexPool, id)
	}

	if s.configPath != "" {
		if err := SaveConfig(s.configPath, s.config); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Index '%s' deleted", id),
	})
}

func (s *Server) handlePutIndexes(w http.ResponseWriter, r *http.Request) {
	var reqList []UpdateIndexSelectionItem
	if err := json.NewDecoder(r.Body).Decode(&reqList); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	updateMap := make(map[string]bool)
	for _, item := range reqList {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if item.Selected != nil {
			updateMap[id] = *item.Selected
		} else if item.DefaultSelected != nil {
			updateMap[id] = *item.DefaultSelected
		}
	}

	for i := range s.config.Indexes {
		if val, ok := updateMap[s.config.Indexes[i].ID]; ok {
			s.config.Indexes[i].DefaultSelected = val
		}
	}

	if s.configPath != "" {
		if err := SaveConfig(s.configPath, s.config); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to save configuration: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
	})
}

// handleEvents handles Server-Sent Events (SSE) connections.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan string, 16)

	s.eventsMu.Lock()
	s.eventClients[clientChan] = true
	s.eventsMu.Unlock()

	defer func() {
		s.eventsMu.Lock()
		if _, exists := s.eventClients[clientChan]; exists {
			delete(s.eventClients, clientChan)
			close(clientChan)
		}
		s.eventsMu.Unlock()
	}()

	fmt.Fprintf(w, "event: connect\ndata: ok\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// HasActiveWebClients returns true if there is at least one SSE client connected.
func (s *Server) HasActiveWebClients() bool {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()
	return len(s.eventClients) > 0
}

// NotifyWebClients broadcasts an action event to all connected SSE clients.
func (s *Server) NotifyWebClients(action string) {
	payload, err := json.Marshal(map[string]string{"action": action})
	if err != nil {
		return
	}
	msg := string(payload)

	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()

	for ch := range s.eventClients {
		select {
		case ch <- msg:
		default:
			// Buffer full; drop message
		}
	}
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

// LaunchURL returns the URL to launch or focus the DocSearch main tab with optional search query and index IDs.
func (s *Server) LaunchURL(query string, indexIDs []string) string {
	s.mu.RLock()
	host := s.config.Server.Host
	port := s.config.Server.Port
	if s.listener != nil {
		if tcpAddr, ok := s.listener.Addr().(*net.TCPAddr); ok {
			port = tcpAddr.Port
			if tcpAddr.IP != nil && !tcpAddr.IP.IsUnspecified() {
				host = tcpAddr.IP.String()
			}
		}
	}
	s.mu.RUnlock()

	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	var params []string
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery != "" {
		params = append(params, "q="+url.QueryEscape(trimmedQuery))
	}

	var escapedIDs []string
	for _, id := range indexIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			escapedIDs = append(escapedIDs, url.QueryEscape(trimmed))
		}
	}
	if len(escapedIDs) > 0 {
		params = append(params, "indexes="+strings.Join(escapedIDs, ","))
	}

	baseURL := fmt.Sprintf("http://%s:%d/launch", host, port)
	if len(params) > 0 {
		return baseURL + "?" + strings.Join(params, "&")
	}
	return baseURL
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

	// Close all SSE client channels
	s.eventsMu.Lock()
	for ch := range s.eventClients {
		delete(s.eventClients, ch)
		close(ch)
	}
	s.eventsMu.Unlock()

	for id, idx := range s.indexPool {
		if err := idx.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close bleve index %s: %w", id, err)
		}
	}
	s.indexPool = make(map[string]bleve.Index)

	return firstErr
}

// Stop is an alias for Close to provide graceful server shutdown.
func (s *Server) Stop() error {
	return s.Close()
}

