package pforward

import (
	"fmt"
	"math/rand"
	"testing"
)

var tlds = []string{"com.", "net.", "org."}

const letters = "abcdefghijklmnopqrstuvwxyz"

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func generateRandomDomain() (domain string) {
	for i := 0; i < rand.Intn(3)+1; i++ {
		domain = fmt.Sprintf("%s.%s", randomString(rand.Intn(9)+1), domain)
	}

	return domain + tlds[rand.Intn(len(tlds))]
}

func generateDomains(n int) []string {
	var domains []string
	for i := 0; i < n; i++ {
		domains = append(domains, generateRandomDomain())
	}
	return domains
}

func TestTrie(t *testing.T) {
	domains := generateDomains(10)
	t.Logf("domains=%+v", domains)

	var root *TrieNode
	for _, domain := range domains {
		root = InsertDomain(domain, root)
	}

	results := Format(root)
	t.Errorf("results=%+v", results)

	for _, domain := range domains {
		if !FindDomainSuffix(domain, root) {
			t.Errorf("domain=%s not match", domain)
		}
	}
}

func TestEdgeCase(t *testing.T) {
	root := InsertDomain(".", nil)

	results := Format(root)
	t.Errorf("results=%+v", results)

	domains := generateDomains(1e6)
	// t.Logf("domains=%+v", domains)
	for _, domain := range domains {
		if !FindDomainSuffix(domain, root) {
			t.Errorf("domain=%s not match", domain)
		}
	}
}
