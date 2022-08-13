package pforward

import (
	"context"
	"fmt"
	"time"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

var log = clog.NewWithPlugin("pforword")

type PForward struct {
	Next        plugin.Handler
	Policy      *Policy
	AutoServer  string
	BlockAAAA   bool
	GeoDatabase *geoip2.Reader
	Timeout     time.Duration
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
	if p.BlockAAAA && question.Qtype == dns.TypeAAAA {
		return dns.RcodeServerFailure, fmt.Errorf("block AAAA response")
	}

	if len(p.AutoServer) > 0 && (question.Qtype == dns.TypeA || question.Qtype == dns.TypeAAAA) {
		return p.AutoServeDNS(ctx, w, r)
	}

	server := p.Policy.SelectServer(question.Name) + ":53"
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

func (p PForward) AutoServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	result114 := queryDNSWithTimeout(ctx, r, "114.114.114.114:53", time.Millisecond*100)
	if result114 == nil {
		return dns.RcodeServerFailure, fmt.Errorf("invalid DNS response")
	}

	var abroad bool
	switch r.Question[0].Qtype {
	case dns.TypeA:
		aResponse := result114.Answer[0].(*dns.A)
		country, err := p.GeoDatabase.Country(aResponse.A)
		if err != nil {
			log.Errorf("[AutoServeDNS] request=%s err=%v", r.String(), err)
			return dns.RcodeServerFailure, err
		}

		if country.Country.IsoCode != "CN" {
			abroad = true
		}
	case dns.TypeAAAA:
		aaaaResponse := result114.Answer[0].(*dns.AAAA)
		country, err := p.GeoDatabase.Country(aaaaResponse.AAAA)
		if err != nil {
			log.Errorf("[AutoServeDNS] request=%s err=%v", r.String(), err)
			return dns.RcodeServerFailure, err
		}

		if country.Country.IsoCode != "CN" {
			abroad = true
		}
	default:
		return dns.RcodeServerFailure, fmt.Errorf("invalid DNS QType")
	}

	result := result114
	if abroad {
		result = queryDNSWithTimeout(ctx, r, p.AutoServer+":53", time.Millisecond*100)
		if result == nil {
			return dns.RcodeServerFailure, fmt.Errorf("DNS query timeout")
		}

		log.Infof("[AutoServeDNS] find abroad domain=%s", r.Question[0].Name)
	}

	if err := w.WriteMsg(result); err != nil {
		log.Errorf("[AutoServeDNS] request=%s response=%s err=%v", r.String(), result.String(), err)
		return dns.RcodeServerFailure, err
	}
	return dns.RcodeSuccess, nil
}

func queryDNSWithTimeout(ctx context.Context, m *dns.Msg, remote string, timeout time.Duration) *dns.Msg {
	var (
		result *dns.Msg
		err    error
		done   = make(chan interface{})
	)
	go func() {
		result, err = dns.Exchange(m, remote)
		done <- new(interface{})
	}()

	t := time.NewTimer(timeout)
	select {
	case <-t.C:
		return nil
	case <-done:
		if err != nil {
			return nil
		}
		return result
	}
}
