package pforward

import (
	"fmt"
	"strings"
)

type TrieNode struct {
	Current  string
	Children map[string]*TrieNode

	End bool
}

func makeSegments(domain string) []string {
	segments := strings.Split(domain, ".")
	for i := 0; i < len(segments)/2; i++ {
		segments[i], segments[len(segments)-1-i] = segments[len(segments)-1-i], segments[i]
	}

	current := 0
	for i := 0; i < len(segments); i++ {
		segments[current] = strings.TrimSpace(segments[i])

		if len(segments[current]) > 0 {
			current += 1
		}
	}

	return segments[:current]
}

func InsertDomain(domain string, root *TrieNode) *TrieNode {
	segments := makeSegments(domain)

	if root == nil {
		root = &TrieNode{Current: "."}
	}
	Insert(segments, root)

	return root
}

func Insert(segments []string, root *TrieNode) {
	current := root
	for _, segment := range segments {
		if current.End {
			break
		}

		if next := current.Children[segment]; next != nil {
			current = next
			continue
		}

		if current.Children == nil {
			current.Children = make(map[string]*TrieNode)
		}
		current.Children[segment] = &TrieNode{Current: segment}

		current = current.Children[segment]
	}

	current.End = true
	current.Children = nil
}

func FindDomainSuffix(domain string, current *TrieNode) bool {
	if current == nil {
		return false
	}
	if current.End {
		return true
	}

	segments := makeSegments(domain)
	for _, segment := range segments {
		if current == nil {
			break
		}

		current = current.Children[segment]
		if current == nil {
			return false
		}
		if current.End {
			return true
		}
	}

	return false
}

func Format(root *TrieNode) []string {
	return dfs(root, "", nil)
}

func dfs(current *TrieNode, domain string, results []string) []string {
	if current == nil {
		return nil
	}
	if current.End {
		if domain == "" {
			return append(results, ".")
		}
		return append(results, domain)
	}

	for prefix, next := range current.Children {
		results = dfs(next, fmt.Sprintf("%s.%s", prefix, domain), results)
	}

	return results
}
