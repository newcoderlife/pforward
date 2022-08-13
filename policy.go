package pforward

import (
	"strings"
	"sync"
)

type Policy struct {
	rules  map[string]string
	server string
	lock   sync.RWMutex
}

func MakePolicy() *Policy {
	return &Policy{
		rules:  make(map[string]string),
		server: "114.114.114.114",
	}
}

func (p *Policy) AddRule(rule string, server string) error {
	if rule == "." {
		p.server = server
		log.Infof("[AddRule] default server=%s", server)
		return nil
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	if len(p.rules[rule]) > 0 {
		log.Warningf("[AddRule] rule=%s already exist", rule)
		return nil
	}

	p.rules[rule] = server
	log.Debugf("[AddRule] add rule=%s server=%s", rule, server)
	return nil
}

func (p *Policy) SelectServer(domain string) string {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for current := domain; len(current) > 0; {
		if server, ok := p.rules[current]; ok {
			log.Debugf("[SelectServer] domain=%s server=%s", domain, server)
			return server
		}

		_, after, found := strings.Cut(current, ".")
		if !found || after == current {
			break
		}
		current = after
	}

	log.Debugf("[SelectServer] domain=%s not match", domain)
	return p.server
}

func (p *Policy) Count() int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return len(p.rules)
}
