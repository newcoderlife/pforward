package rule

import (
	"fmt"
	"strings"
)

type Set struct {
	Rules []*Rule
}

func (s *Set) Append(raw string, upstream string) error {
	if strings.HasPrefix(raw, "regex:") {
		s.Rules = append(s.Rules, &Rule{Pattern: strings.TrimPrefix(raw, "regex:"), Type: RegexRule, Upstream: upstream})
	} else if strings.HasPrefix(raw, "suffix:") {
		s.Rules = append(s.Rules, &Rule{Pattern: strings.TrimPrefix(raw, "suffix:"), Type: SuffixRule, Upstream: upstream})
	} else if strings.HasPrefix(raw, "keyword:") {
		s.Rules = append(s.Rules, &Rule{Pattern: strings.TrimPrefix(raw, "keyword:"), Type: SuffixRule, Upstream: upstream})
	} else {
		return fmt.Errorf("invalid rule=%s", raw)
	}
	return nil
}

func (s Set) Match(content string) string {
	for _, rule := range s.Rules {
		if rule.Match(content) {
			return rule.Upstream
		}
	}
	return ""
}
