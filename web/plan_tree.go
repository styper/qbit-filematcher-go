package web

import (
	"path"
	"sort"
	"strings"
)

// planNode is one entry in the mapped_files tree under save_path.
type planNode struct {
	Name     string
	Children []*planNode
}

// buildPlanTree turns slash-separated mapped_files paths into a sorted tree.
// Empty entries (pad files) are skipped.
func buildPlanTree(mapped []string) []*planNode {
	root := &planNode{}
	for _, entry := range mapped {
		entry = path.Clean("/" + strings.ReplaceAll(entry, "\\", "/"))
		entry = strings.TrimPrefix(entry, "/")
		if entry == "" || entry == "." {
			continue
		}
		insertPlanNode(root, strings.Split(entry, "/"))
	}
	sortPlanTree(root)
	return root.Children
}

func insertPlanNode(parent *planNode, parts []string) {
	if len(parts) == 0 || parts[0] == "" {
		return
	}
	name := parts[0]
	var child *planNode
	for _, c := range parent.Children {
		if c.Name == name {
			child = c
			break
		}
	}
	if child == nil {
		child = &planNode{Name: name}
		parent.Children = append(parent.Children, child)
	}
	if len(parts) == 1 {
		return
	}
	insertPlanNode(child, parts[1:])
}

func sortPlanTree(n *planNode) {
	if n == nil || len(n.Children) == 0 {
		return
	}
	sort.Slice(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		aDir, bDir := len(a.Children) > 0, len(b.Children) > 0
		if aDir != bDir {
			return aDir // directories first
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	for _, c := range n.Children {
		sortPlanTree(c)
	}
}
