package rule

import (
	"regexp"
	"strings"
)

type RuleType int

const (
	RegexRule RuleType = iota
	SuffixRule
	KeywordRule
)

type Rule struct {
	Pattern  string
	Type     RuleType
	Upstream string
}

func (r Rule) Match(content string) bool {
	switch r.Type {
	case RegexRule:
		matched, err := regexp.MatchString(r.Pattern, content)
		return err == nil && matched
	case SuffixRule:
		return strings.HasSuffix(content, r.Pattern)
	case KeywordRule:
		return strings.Contains(content, r.Pattern)
	}
	return false
}
