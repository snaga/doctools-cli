package pdf

import (
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractText extracts text content from a PDF file using go-fitz.
func ExtractText(inputPath string, outputPath string, startPage int, endPage int) (string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputPath)
	}

	doc, err := fitz.New(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open pdf file: %w", err)
	}
	defer doc.Close()

	totalPages := doc.NumPage()
	if totalPages == 0 {
		return "(No text content extracted)", nil
	}

	if startPage < 1 {
		startPage = 1
	}
	if endPage <= 0 || endPage > totalPages {
		endPage = totalPages
	}
	if startPage > totalPages || startPage > endPage {
		return "", fmt.Errorf("invalid page range %d-%d for pdf with %d pages", startPage, endPage, totalPages)
	}

	var contentBuilder strings.Builder
	hasContent := false
	for p := startPage; p <= endPage; p++ {
		text, err := doc.Text(p - 1)
		if err != nil {
			return "", fmt.Errorf("failed to extract text from page %d: %w", p, err)
		}
		if strings.TrimSpace(text) != "" {
			hasContent = true
		}
		contentBuilder.WriteString(fmt.Sprintf("--- Page %d ---\n", p))
		contentBuilder.WriteString(text)
		contentBuilder.WriteString("\n\n")
	}

	var resText string
	if !hasContent {
		resText = "(No text content extracted)"
	} else {
		resText = contentBuilder.String()
		if strings.TrimSpace(resText) == "" {
			resText = "(No text content extracted)"
		}
	}

	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputPath, []byte(resText), 0644); err != nil {
			return "", fmt.Errorf("failed to write output file: %w", err)
		}
	}

	return resText, nil
}

// Split extracts specified page range from inputPath to outputPath.
func Split(inputPath string, outputPath string, startPage int, endPage int) (string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputPath)
	}

	conf := model.NewDefaultConfiguration()
	pageSpan := fmt.Sprintf("%d-%d", startPage, endPage)
	if startPage <= 0 || endPage < startPage {
		return "", fmt.Errorf("invalid page range %d-%d", startPage, endPage)
	}

	tmpDir, err := os.MkdirTemp("", "pdf_split_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	err = api.TrimFile(inputPath, filepath.Join(tmpDir, "out.pdf"), []string{pageSpan}, conf)
	if err != nil {
		return "", fmt.Errorf("failed to trim pdf: %w", err)
	}

	trimmedPath := filepath.Join(tmpDir, "out.pdf")
	if outputPath == "" {
		baseName := filepath.Base(inputPath)
		ext := filepath.Ext(baseName)
		outputPath = filepath.Join(filepath.Dir(inputPath), fmt.Sprintf("%s_p%d-p%d%s", baseName[:len(baseName)-len(ext)], startPage, endPage, ext))
	}

	data, err := os.ReadFile(trimmedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read trimmed pdf: %w", err)
	}

	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write output pdf: %w", err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

// Merge merges multiple PDF files into outputPath.
func Merge(inputPaths []string, outputPath string) (string, error) {
	if len(inputPaths) == 0 {
		return "", fmt.Errorf("input paths list is empty")
	}

	for _, p := range inputPaths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return "", fmt.Errorf("input file not found: %s", p)
		}
	}

	conf := model.NewDefaultConfiguration()
	err := api.MergeCreateFile(inputPaths, outputPath, false, conf)
	if err != nil {
		return "", fmt.Errorf("failed to merge pdf files: %w", err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

// ExtractImages extracts images from PDF using pdfcpu API.
func ExtractImages(inputPath string, outputDir string, selectedPages []int) ([]string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	if outputDir == "" {
		baseName := filepath.Base(inputPath)
		ext := filepath.Ext(baseName)
		outputDir = filepath.Join(filepath.Dir(inputPath), baseName[:len(baseName)-len(ext)]+"_images")
	}
	_ = os.MkdirAll(outputDir, 0755)

	conf := model.NewDefaultConfiguration()
	var pages []string
	for _, p := range selectedPages {
		pages = append(pages, strconv.Itoa(p))
	}

	err := api.ExtractImagesFile(inputPath, outputDir, pages, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to extract images: %w", err)
	}

	var outputPaths []string
	entries, _ := os.ReadDir(outputDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			abs, err := filepath.Abs(filepath.Join(outputDir, entry.Name()))
			if err == nil {
				outputPaths = append(outputPaths, abs)
			}
		}
	}

	return outputPaths, nil
}

// ExtractPages renders PDF pages as images (PNG/JPG) using go-fitz.
func ExtractPages(inputPath string, outputDir string, dpi int, format string, startPage int, endPage int, force bool) ([]string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", inputPath)
	}

	if dpi <= 0 {
		dpi = 150
	}

	format = strings.ToLower(format)
	if format == "jpeg" {
		format = "jpg"
	}
	if format != "png" && format != "jpg" {
		return nil, fmt.Errorf("unsupported image format: %s (must be png or jpg)", format)
	}

	doc, err := fitz.New(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pdf file: %w", err)
	}
	defer doc.Close()

	totalPages := doc.NumPage()
	if totalPages == 0 {
		return nil, fmt.Errorf("pdf file has no pages")
	}

	if startPage < 1 {
		startPage = 1
	}
	if endPage <= 0 || endPage > totalPages {
		endPage = totalPages
	}
	if startPage > totalPages || startPage > endPage {
		return nil, fmt.Errorf("invalid page range %d-%d for pdf with %d pages", startPage, endPage, totalPages)
	}

	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	stem := baseName[:len(baseName)-len(ext)]

	digits := len(strconv.Itoa(totalPages))
	if digits < 2 {
		digits = 2
	}
	pageFmt := fmt.Sprintf("%%0%dd", digits)

	var outputPaths []string
	for p := startPage; p <= endPage; p++ {
		fileName := fmt.Sprintf("%s_page_"+pageFmt+".%s", stem, p, format)
		outputPath := filepath.Join(outputDir, fileName)

		if !force {
			if _, err := os.Stat(outputPath); err == nil {
				return nil, fmt.Errorf("output file already exists: %s", outputPath)
			}
		}

		img, err := doc.ImageDPI(p-1, float64(dpi))
		if err != nil {
			return nil, fmt.Errorf("failed to render page %d: %w", p, err)
		}

		outFile, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create image file %s: %w", outputPath, err)
		}

		if format == "png" {
			err = png.Encode(outFile, img)
		} else {
			err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
		}
		closeErr := outFile.Close()
		if err == nil {
			err = closeErr
		}

		if err != nil {
			return nil, fmt.Errorf("failed to encode image page %d: %w", p, err)
		}

		absPath, err := filepath.Abs(outputPath)
		if err != nil {
			absPath = outputPath
		}
		outputPaths = append(outputPaths, absPath)
	}

	return outputPaths, nil
}

