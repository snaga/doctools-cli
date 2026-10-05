package fts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/gen2brain/go-fitz"
	"github.com/xuri/excelize/v2"
)

// DocumentChunk represents a searchable document chunk.
type DocumentChunk struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"file_path"`
	FileName    string    `json:"file_name"`
	FileType    string    `json:"file_type"`
	UpdatedAt   time.Time `json:"updated_at"`
	Content     string    `json:"content"`
	UnitType    string    `json:"unit_type"`
	UnitName    string    `json:"unit_name"`
	PageOrIndex int       `json:"page_or_index"`
	Locator     string    `json:"locator"`
}

// BuildOptions holds configuration options for building full text search index.
type BuildOptions struct {
	IndexPath   string
	Timeout     time.Duration
	Force       bool
	Verbose     bool
	IncludeExts []string
	ExcludeExts []string
	IncludeDirs []string
	ExcludeDirs []string
}

func normalizeExt(ext string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// NormalizeExt normalizes file extension string with leading dot, lowercase, and trimmed spaces.
func NormalizeExt(ext string) string {
	return normalizeExt(ext)
}

func normalizeDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	dir = filepath.ToSlash(filepath.Clean(dir))
	dir = strings.TrimPrefix(dir, "./")
	dir = strings.TrimPrefix(dir, "/")
	dir = strings.TrimSuffix(dir, "/")
	return dir
}

// NormalizeDir normalizes directory path by trimming spaces, converting separators to slash, and removing leading/trailing slashes.
func NormalizeDir(dir string) string {
	return normalizeDir(dir)
}

// BuildResult represents the result metrics of building an index.
type BuildResult struct {
	IndexPath     string `json:"index_path"`
	IndexedFiles  int    `json:"indexed_files"`
	SkippedFiles  int    `json:"skipped_files"`
	TimeoutFiles  int    `json:"timeout_files"`
	IndexedChunks int    `json:"indexed_chunks"`
	TimeMs        int64  `json:"time_ms"`
}

// SourceMeta represents the source file metadata in search hit.
type SourceMeta struct {
	FilePath  string    `json:"file_path"`
	FileName  string    `json:"file_name"`
	FileType  string    `json:"file_type"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TargetMeta represents the document chunk location metadata in search hit.
type TargetMeta struct {
	UnitType    string `json:"unit_type"`
	UnitName    string `json:"unit_name"`
	PageOrIndex int    `json:"page_or_index"`
	Locator     string `json:"locator"`
}

// Hit represents a single search match.
type Hit struct {
	Score   float64    `json:"score"`
	Snippet string     `json:"snippet"`
	Source  SourceMeta `json:"source"`
	Target  TargetMeta `json:"target"`
}

// QueryResult represents the search query results.
type QueryResult struct {
	TotalHits int   `json:"total_hits"`
	Hits      []Hit `json:"hits"`
}

func buildIndexMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	contentMapping := bleve.NewTextFieldMapping()
	contentMapping.Store = true
	docMapping.AddFieldMappingsAt("content", contentMapping)

	filePathMapping := bleve.NewTextFieldMapping()
	filePathMapping.Analyzer = "keyword"
	filePathMapping.Store = true
	docMapping.AddFieldMappingsAt("file_path", filePathMapping)

	for _, field := range []string{"id", "file_name", "file_type", "unit_type", "unit_name", "locator"} {
		m := bleve.NewTextFieldMapping()
		m.Store = true
		docMapping.AddFieldMappingsAt(field, m)
	}

	dateMapping := bleve.NewDateTimeFieldMapping()
	dateMapping.Store = true
	docMapping.AddFieldMappingsAt("updated_at", dateMapping)

	numMapping := bleve.NewNumericFieldMapping()
	numMapping.Store = true
	docMapping.AddFieldMappingsAt("page_or_index", numMapping)

	indexMapping.DefaultMapping = docMapping
	return indexMapping
}

func parseTimeVal(val interface{}) (time.Time, bool) {
	if val == nil {
		return time.Time{}, false
	}
	if t, ok := val.(time.Time); ok {
		return t, true
	}
	if str, ok := val.(string); ok {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, str); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func cleanPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return filepath.ToSlash(filepath.Clean(abs))
}

func normalizePath(path string) string {
	return strings.ToLower(cleanPath(path))
}

// BuildIndexWithOptions builds a Bleve full text search index with custom options.
func BuildIndexWithOptions(sourceDir string, opts BuildOptions) (*BuildResult, error) {
	startTime := time.Now()

	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("source directory not found: %s", sourceDir)
	}

	indexPath := opts.IndexPath
	if indexPath == "" {
		indexPath = filepath.Join(sourceDir, "fts.bleve")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}

	var index bleve.Index
	var err error

	if opts.Force {
		if _, err := os.Stat(indexPath); err == nil {
			_ = os.RemoveAll(indexPath)
		}
		index, err = bleve.New(indexPath, buildIndexMapping())
		if err != nil {
			return nil, fmt.Errorf("failed to create bleve index: %w", err)
		}
	} else {
		if _, err := os.Stat(indexPath); err == nil {
			index, err = bleve.Open(indexPath)
			if err != nil {
				_ = os.RemoveAll(indexPath)
				index, err = bleve.New(indexPath, buildIndexMapping())
				if err != nil {
					return nil, fmt.Errorf("failed to create bleve index: %w", err)
				}
			}
		} else {
			index, err = bleve.New(indexPath, buildIndexMapping())
			if err != nil {
				return nil, fmt.Errorf("failed to create bleve index: %w", err)
			}
		}
	}
	defer index.Close()

	var incExts []string
	if len(opts.IncludeExts) == 0 {
		incExts = []string{".xlsx", ".pptx", ".pdf", ".docx"}
	} else {
		incExts = opts.IncludeExts
	}

	incSet := make(map[string]bool)
	for _, e := range incExts {
		if norm := normalizeExt(e); norm != "" {
			incSet[norm] = true
		}
	}

	excSet := make(map[string]bool)
	for _, e := range opts.ExcludeExts {
		if norm := normalizeExt(e); norm != "" {
			excSet[norm] = true
		}
	}

	var incDirs []string
	for _, d := range opts.IncludeDirs {
		if norm := normalizeDir(d); norm != "" {
			incDirs = append(incDirs, norm)
		}
	}

	var excDirs []string
	for _, d := range opts.ExcludeDirs {
		if norm := normalizeDir(d); norm != "" {
			excDirs = append(excDirs, norm)
		}
	}

	isDirExcluded := func(dirRel string, dirName string) bool {
		for _, exc := range excDirs {
			if strings.EqualFold(dirName, exc) || strings.EqualFold(dirRel, exc) || strings.HasPrefix(strings.ToLower(dirRel), strings.ToLower(exc)+"/") {
				return true
			}
		}
		return false
	}

	isDirIncludedOrAncestorOrDescendant := func(dirRel string) bool {
		if len(incDirs) == 0 {
			return true
		}
		dirRelLower := strings.ToLower(dirRel)
		for _, inc := range incDirs {
			incLower := strings.ToLower(inc)
			// Matches exact, is descendant of inc, or is ancestor of inc
			if dirRelLower == incLower || strings.HasPrefix(dirRelLower, incLower+"/") || strings.HasPrefix(incLower, dirRelLower+"/") {
				return true
			}
		}
		return false
	}

	isFileIncluded := func(fileRel string) bool {
		if len(incDirs) == 0 {
			return true
		}
		fileRelLower := strings.ToLower(filepath.ToSlash(fileRel))
		for _, inc := range incDirs {
			incLower := strings.ToLower(inc)
			if strings.HasPrefix(fileRelLower, incLower+"/") || fileRelLower == incLower {
				return true
			}
		}
		return false
	}

	var indexedFiles, skippedFiles, timeoutFiles, indexedChunks int

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		rel, relErr := filepath.Rel(sourceDir, path)
		if relErr != nil {
			rel = path
		}
		relSlash := filepath.ToSlash(rel)

		if info.IsDir() {
			// Root directory itself is not skipped
			if relSlash == "." || relSlash == "" {
				return nil
			}

			// Index directory check (never index Bleve index folder)
			absPath := cleanPath(path)
			absIdx := cleanPath(indexPath)
			if strings.EqualFold(absPath, absIdx) || strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(absIdx)+"/") {
				return filepath.SkipDir
			}

			name := info.Name()

			// Check if explicitly included in IncludeDirs (exact or ancestor/descendant)
			var explicitlyIncluded bool
			if len(incDirs) > 0 {
				explicitlyIncluded = isDirIncludedOrAncestorOrDescendant(relSlash)
			}

			// Hidden directory check (.trash, .obsidian, .git, etc.)
			if strings.HasPrefix(name, ".") {
				if !explicitlyIncluded {
					return filepath.SkipDir
				}
			}

			// ExcludeDirs check
			if isDirExcluded(relSlash, name) {
				return filepath.SkipDir
			}

			// IncludeDirs check
			if len(incDirs) > 0 && !explicitlyIncluded {
				return filepath.SkipDir
			}

			return nil
		}

		// File processing
		absPath := cleanPath(path)
		absIdx := cleanPath(indexPath)

		if strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(absIdx)) {
			return nil
		}

		// If IncludeDirs is specified, verify file is under an included dir
		if !isFileIncluded(relSlash) {
			return nil
		}

		ext := normalizeExt(filepath.Ext(path))
		if ext == "" {
			return nil
		}

		if excSet[ext] {
			return nil
		}

		if !incSet[ext] {
			return nil
		}

		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "[FTS] Processing file: %s ...\n", rel)
		}

		if !opts.Force {
			tq := bleve.NewMatchQuery(absPath)
			tq.SetField("file_path")
			req := bleve.NewSearchRequestOptions(tq, 100, 0, false)
			req.Fields = []string{"updated_at", "id"}
			res, searchErr := index.Search(req)
			if searchErr == nil && res.Total > 0 {
				existingTime, hasTime := parseTimeVal(res.Hits[0].Fields["updated_at"])
				if hasTime && info.ModTime().Unix() <= existingTime.Unix() {
					skippedFiles++
					if opts.Verbose {
						fmt.Fprintf(os.Stderr, "[FTS] Skipped (unchanged): %s\n", rel)
					}
					return nil
				}

				batch := index.NewBatch()
				for _, h := range res.Hits {
					batch.Delete(h.ID)
				}
				_ = index.Batch(batch)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()

		// A budget this small cannot cover even opening the file, so report the
		// timeout without starting a goroutine. It also keeps the outcome
		// deterministic: below a millisecond, whether the context timer fires
		// before the parse finishes is a coin flip.
		if opts.Timeout <= 1*time.Millisecond || ctx.Err() != nil {
			timeoutFiles++
			log.Printf("Warning: parsing timeout for file %s", path)
			return nil
		}

		type parseResult struct {
			chunks []DocumentChunk
			err    error
		}
		ch := make(chan parseResult, 1)

		parser := getParseFileImpl()
		go func() {
			chunks, pErr := parser(ctx, absPath, rel, info, ext)
			ch <- parseResult{chunks: chunks, err: pErr}
		}()

		select {
		case <-ctx.Done():
			timeoutFiles++
			log.Printf("Warning: parsing timeout for file %s", path)
			return nil
		case res := <-ch:
			if ctx.Err() != nil {
				timeoutFiles++
				log.Printf("Warning: parsing timeout for file %s", path)
				return nil
			}
			if res.err != nil {
				if errors.Is(res.err, context.DeadlineExceeded) || errors.Is(res.err, context.Canceled) {
					timeoutFiles++
					log.Printf("Warning: parsing timeout for file %s", path)
				} else {
					log.Printf("Warning: failed to parse file %s: %v", path, res.err)
				}
				return nil
			}
			if len(res.chunks) > 0 {
				batch := index.NewBatch()
				for _, chunk := range res.chunks {
					_ = batch.Index(chunk.ID, chunk)
				}
				if err := index.Batch(batch); err != nil {
					log.Printf("Warning: failed to index batch for %s: %v", path, err)
					return nil
				}
				indexedFiles++
				indexedChunks += len(res.chunks)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to build index: %w", err)
	}

	absIndexPath, err := filepath.Abs(indexPath)
	if err != nil {
		absIndexPath = indexPath
	}

	return &BuildResult{
		IndexPath:     absIndexPath,
		IndexedFiles:  indexedFiles,
		SkippedFiles:  skippedFiles,
		TimeoutFiles:  timeoutFiles,
		IndexedChunks: indexedChunks,
		TimeMs:        time.Since(startTime).Milliseconds(),
	}, nil
}

// BuildIndex builds a Bleve full text search index from files in sourceDir.
func BuildIndex(sourceDir string, indexPath string) (string, error) {
	res, err := BuildIndexWithOptions(sourceDir, BuildOptions{
		IndexPath: indexPath,
		Timeout:   10 * time.Second,
		Force:     true,
	})
	if err != nil {
		return "", err
	}
	return res.IndexPath, nil
}

// parseFileImpl is a variable so that tests can drive the timeout branches of
// the walk function with a parser that is slow on demand.
var (
	parseFileMu   sync.RWMutex
	parseFileImpl = parseFile
)

func getParseFileImpl() func(context.Context, string, string, os.FileInfo, string) ([]DocumentChunk, error) {
	parseFileMu.RLock()
	defer parseFileMu.RUnlock()
	return parseFileImpl
}

func setParseFileImpl(fn func(context.Context, string, string, os.FileInfo, string) ([]DocumentChunk, error)) {
	parseFileMu.Lock()
	defer parseFileMu.Unlock()
	parseFileImpl = fn
}

func parseFile(ctx context.Context, absPath string, relPath string, info os.FileInfo, ext string) ([]DocumentChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absPath = cleanPath(absPath)
	switch ext {
	case ".xlsx":
		return parseXlsx(absPath, relPath, info)
	case ".pptx":
		return parsePptx(absPath, relPath, info)
	case ".pdf":
		return parsePdf(absPath, relPath, info)
	case ".csv", ".txt", ".md", ".json", ".html":
		return parseTextFile(absPath, relPath, info, ext)
	default:
		return nil, nil
	}
}

func parseXlsx(absPath string, relPath string, info os.FileInfo) ([]DocumentChunk, error) {
	f, err := excelize.OpenFile(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var chunks []DocumentChunk
	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		var rowStrs []string
		for _, row := range rows {
			var cellStrs []string
			for _, cell := range row {
				if trimmed := strings.TrimSpace(cell); trimmed != "" {
					cellStrs = append(cellStrs, trimmed)
				}
			}
			if len(cellStrs) > 0 {
				rowStrs = append(rowStrs, strings.Join(cellStrs, " "))
			}
		}
		content := strings.Join(rowStrs, "\n")
		locator := fmt.Sprintf("sheet=%s", sheet)
		chunks = append(chunks, DocumentChunk{
			ID:          fmt.Sprintf("%s#%s", filepath.ToSlash(relPath), locator),
			FilePath:    absPath,
			FileName:    info.Name(),
			FileType:    ".xlsx",
			UpdatedAt:   info.ModTime(),
			Content:     content,
			UnitType:    "sheet",
			UnitName:    sheet,
			PageOrIndex: 0,
			Locator:     locator,
		})
	}
	return chunks, nil
}

func parsePptx(absPath string, relPath string, info os.FileInfo) ([]DocumentChunk, error) {
	r, err := zip.OpenReader(absPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	slideFiles := make(map[int]*zip.File)
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			numStr := strings.TrimPrefix(f.Name, "ppt/slides/slide")
			numStr = strings.TrimSuffix(numStr, ".xml")
			if num, err := strconv.Atoi(numStr); err == nil {
				slideFiles[num] = f
			}
		}
	}

	var slideNums []int
	for num := range slideFiles {
		slideNums = append(slideNums, num)
	}
	sort.Ints(slideNums)

	var chunks []DocumentChunk
	for _, sNum := range slideNums {
		f := slideFiles[sNum]
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		texts := parseXmlTexts(data)
		content := strings.Join(texts, "\n")
		locator := fmt.Sprintf("slide=%d", sNum)
		chunks = append(chunks, DocumentChunk{
			ID:          fmt.Sprintf("%s#%s", filepath.ToSlash(relPath), locator),
			FilePath:    absPath,
			FileName:    info.Name(),
			FileType:    ".pptx",
			UpdatedAt:   info.ModTime(),
			Content:     content,
			UnitType:    "slide",
			UnitName:    "",
			PageOrIndex: sNum,
			Locator:     locator,
		})
	}
	return chunks, nil
}

func parseXmlTexts(data []byte) []string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var texts []string
	var inText bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				if str := strings.TrimSpace(string(t)); str != "" {
					texts = append(texts, str)
				}
			}
		}
	}
	return texts
}

func parsePdf(absPath string, relPath string, info os.FileInfo) ([]DocumentChunk, error) {
	doc, err := fitz.New(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pdf: %w", err)
	}
	defer doc.Close()

	pageCount := doc.NumPage()
	if pageCount == 0 {
		pageCount = 1
	}

	var chunks []DocumentChunk
	for page := 1; page <= pageCount; page++ {
		text, _ := doc.Text(page - 1)
		locator := fmt.Sprintf("page=%d", page)
		chunks = append(chunks, DocumentChunk{
			ID:          fmt.Sprintf("%s#%s", filepath.ToSlash(relPath), locator),
			FilePath:    absPath,
			FileName:    info.Name(),
			FileType:    ".pdf",
			UpdatedAt:   info.ModTime(),
			Content:     text,
			UnitType:    "page",
			UnitName:    "",
			PageOrIndex: page,
			Locator:     locator,
		})
	}
	return chunks, nil
}

func parseTextFile(absPath string, relPath string, info os.FileInfo, ext string) ([]DocumentChunk, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	chunk := DocumentChunk{
		ID:          filepath.ToSlash(relPath),
		FilePath:    absPath,
		FileName:    info.Name(),
		FileType:    ext,
		UpdatedAt:   info.ModTime(),
		Content:     string(data),
		UnitType:    "section",
		UnitName:    "",
		PageOrIndex: 0,
		Locator:     "",
	}
	return []DocumentChunk{chunk}, nil
}

// QueryIndex searches Bleve index with query string.
func QueryIndex(indexPath string, queryString string, limit int) (*QueryResult, error) {
	index, err := bleve.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bleve index: %w", err)
	}
	defer index.Close()

	if limit <= 0 {
		limit = 10
	}

	query := bleve.NewQueryStringQuery(queryString)
	searchRequest := bleve.NewSearchRequestOptions(query, limit, 0, false)
	searchRequest.Highlight = bleve.NewHighlight()
	searchRequest.Fields = []string{"*"}

	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	hits := make([]Hit, 0, len(searchResult.Hits))
	for _, match := range searchResult.Hits {
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
		unitType, _ := match.Fields["unit_type"].(string)
		unitName, _ := match.Fields["unit_name"].(string)
		locator, _ := match.Fields["locator"].(string)

		updatedAt, _ := parseTimeVal(match.Fields["updated_at"])

		var pageOrIndex int
		if poiVal, ok := match.Fields["page_or_index"].(float64); ok {
			pageOrIndex = int(poiVal)
		} else if poiValInt, ok := match.Fields["page_or_index"].(int); ok {
			pageOrIndex = poiValInt
		}

		hits = append(hits, Hit{
			Score:   match.Score,
			Snippet: snippet,
			Source: SourceMeta{
				FilePath:  filePath,
				FileName:  fileName,
				FileType:  fileType,
				UpdatedAt: updatedAt,
			},
			Target: TargetMeta{
				UnitType:    unitType,
				UnitName:    unitName,
				PageOrIndex: pageOrIndex,
				Locator:     locator,
			},
		})
	}

	return &QueryResult{
		TotalHits: int(searchResult.Total),
		Hits:      hits,
	}, nil
}
