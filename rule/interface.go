package rule

import (
	"fmt"
	"strings"

	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var (
	log = clog.NewWithPlugin("pforward")
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
		s.Rules = append(s.Rules, &Rule{Pattern: strings.TrimPrefix(raw, "keyword:"), Type: KeywordRule, Upstream: upstream})
	} else {
		return fmt.Errorf("invalid rule=%s", raw)
	}
	return nil
}

func (s Set) Match(content string) string {
	for _, rule := range s.Rules {
		if rule.Match(content) {
			log.Debugf("domain=%s rule=%+v", content, rule)
			return rule.Upstream
		}
	}
	log.Debugf("domain=%s fallthrough", content)
	return ""
}
