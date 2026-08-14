package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type aText struct {
	Text string `xml:",chardata"`
}

type aP struct {
	Texts []aText `xml:"r>t"`
}

type sld struct {
	XMLName xml.Name `xml:"sld"`
	Texts   []string `xml:"cSld>spTree>sp>txBody>p>r>t"`
}

// ExtractTextPureGo extracts slide text from PPTX archive using pure Go zip+xml parsing.
func ExtractTextPureGo(inputPath string, outputPath string, startSlide int, endSlide int) (string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputPath)
	}

	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open pptx zip: %w", err)
	}
	defer r.Close()

	// Find slide XML files (ppt/slides/slideX.xml)
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

	totalSlides := len(slideFiles)
	if totalSlides == 0 {
		return "", fmt.Errorf("no slides found in pptx")
	}

	if endSlide == 0 || endSlide > totalSlides {
		endSlide = totalSlides
	}
	if startSlide < 1 {
		startSlide = 1
	}

	var slideNums []int
	for num := range slideFiles {
		if num >= startSlide && num <= endSlide {
			slideNums = append(slideNums, num)
		}
	}
	sort.Ints(slideNums)

	var mdLines []string

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

		var s sld
		if err := xml.Unmarshal(data, &s); err != nil {
			// Fallback token parsing if struct unmarshal incomplete
			texts := parseTokens(data)
			s.Texts = texts
		}

		mdLines = append(mdLines, fmt.Sprintf("## Slide %d", sNum))
		for _, t := range s.Texts {
			tTrim := strings.TrimSpace(t)
			if tTrim != "" {
				mdLines = append(mdLines, fmt.Sprintf("- %s", tTrim))
			}
		}
		mdLines = append(mdLines, "")
	}

	content := strings.Join(mdLines, "\n")

	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err == nil {
			_ = os.WriteFile(outputPath, []byte(content), 0644)
		}
		abs, err := filepath.Abs(outputPath)
		if err == nil {
			outputPath = abs
		}
	}

	return content, nil
}

func parseTokens(data []byte) []string {
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
				texts = append(texts, string(t))
			}
		}
	}
	return texts
}
