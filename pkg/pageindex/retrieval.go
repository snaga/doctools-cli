package pageindex

import (
	"encoding/json"
	"fmt"
	"os"

	"doctools-cli/pkg/excel"
	"doctools-cli/pkg/pdf"
	"doctools-cli/pkg/pptx"
)

const PageIndexExt = ".pageindex.json"

// TreeNode represents a PageIndex node.
type TreeNode struct {
	NodeID     string                 `json:"node_id"`
	Title      string                 `json:"title,omitempty"`
	StartIndex int                    `json:"start_index,omitempty"`
	EndIndex   int                    `json:"end_index,omitempty"`
	SheetName  string                 `json:"sheet_name,omitempty"`
	Nodes      []TreeNode             `json:"nodes,omitempty"`
	Extra      map[string]interface{} `json:"-"`
}

// GetTree loads pageindex json and filters tree by node_id and depth.
func GetTree(inputPath string, nodeID string, depth int) (map[string]interface{}, error) {
	indexPath := inputPath + PageIndexExt
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("PageIndex file not found: %s", indexPath)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("failed to parse pageindex json: %w", err)
	}

	if nodeID != "" {
		found := findNodeMap(root, nodeID)
		if found == nil {
			return nil, fmt.Errorf("Node ID '%s' not found", nodeID)
		}
		root = found
	}

	if depth <= 0 {
		depth = 2
	}
	filtered := filterTreeMap(root, depth)

	return filtered, nil
}

func findNodeMap(node map[string]interface{}, nodeID string) map[string]interface{} {
	if id, ok := node["node_id"].(string); ok && id == nodeID {
		return node
	}

	if nodes, ok := node["nodes"].([]interface{}); ok {
		for _, child := range nodes {
			if childMap, ok := child.(map[string]interface{}); ok {
				res := findNodeMap(childMap, nodeID)
				if res != nil {
					return res
				}
			}
		}
	}
	return nil
}

func filterTreeMap(node map[string]interface{}, depth int) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range node {
		if k != "nodes" {
			res[k] = v
		}
	}

	if depth > 0 {
		if nodes, ok := node["nodes"].([]interface{}); ok {
			var newNodes []map[string]interface{}
			for _, child := range nodes {
				if childMap, ok := child.(map[string]interface{}); ok {
					newNodes = append(newNodes, filterTreeMap(childMap, depth-1))
				}
			}
			res["nodes"] = newNodes
		}
	}

	return res
}

// GetContent extracts content from PDF, PPTX, or XLSX file based on pageindex node or parameters.
func GetContent(filePath string, nodeType string, nodeID string, startPage int, endPage int, sheetName string) (string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	if nodeID != "" {
		indexPath := filePath + PageIndexExt
		b, err := os.ReadFile(indexPath)
		if err == nil {
			var root map[string]interface{}
			_ = json.Unmarshal(b, &root)
			node := findNodeMap(root, nodeID)
			if node != nil {
				if s, ok := node["start_index"].(float64); ok {
					startPage = int(s)
				}
				if e, ok := node["end_index"].(float64); ok {
					endPage = int(e)
				}
				if sn, ok := node["sheet_name"].(string); ok {
					sheetName = sn
				} else if sn, ok := node["node_id"].(string); ok {
					sheetName = sn
				}
			}
		}
	}

	switch nodeType {
	case "pdf":
		return pdf.ExtractText(filePath, "", startPage, endPage)
	case "pptx":
		return pptx.ExtractTextPureGo(filePath, "", startPage, endPage)
	case "xlsx", "excel":
		outPaths, err := excel.ExtractCSV(filePath, "", []string{sheetName}, "utf-8")
		if err != nil {
			return "", err
		}
		if len(outPaths) > 0 {
			b, err := os.ReadFile(outPaths[0])
			if err == nil {
				return string(b), nil
			}
		}
		return "", nil
	default:
		return "", fmt.Errorf("unsupported node type: %s", nodeType)
	}
}
