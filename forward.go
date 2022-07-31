package pforward

import (
	"context"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
)

var log = clog.NewWithPlugin("pforword")

type PForward struct {
	Next plugin.Handler

	policy *Policy
}

func MakePForward(next plugin.Handler, policy *Policy) plugin.Handler {
	return &PForward{
		Next:   next,
		policy: policy,
	}
}

func (PForward) Name() string {
	return "pforward"
}

// ServeDNS judges request and go different upstreams
func (p PForward) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Recovered. Error:%v", r)
		}
	}()
	question := r.Question[0]

	server := p.policy.SelectServer(question.Name) + ":53"
	result, err := dns.Exchange(r, server)
	if err != nil {
		log.Errorf("[ServeDNS] request=%s Exchange err=%v", r.String(), err)
		return dns.RcodeServerFailure, err
	}

	if err := w.WriteMsg(result); err != nil {
		log.Errorf("[ServeDNS] request=%s response=%s WriteMsg err=%v", r.String(), result.String(), err)
		return dns.RcodeServerFailure, err
	}

	return dns.RcodeSuccess, nil
}
