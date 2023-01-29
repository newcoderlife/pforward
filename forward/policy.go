package forward

import (
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/newcoderlife/pforward/geo"
	"github.com/newcoderlife/pforward/rule"
	"github.com/newcoderlife/pforward/upstream"
)

type Policy struct {
	GEO     *geo.GEO
	Rule    *rule.Set
	Default string
}

// handleRequestPolicy 根据规则文件判断上游，未命中则返回默认
func (p Policy) handleRequestPolicy(request *dns.Msg) string {
	return p.Rule.Match(parseDomain(request))
}

// handleResponsePolicy 根据 GEO 策略判断上游，未命中则返回默认
func (p Policy) handleResponsePolicy(response *dns.Msg) string {
	if response == nil || len(response.Answer) == 0 {
		return p.Default
	}

	for _, answer := range response.Answer {
		var addr net.IP
		switch v := answer.(type) {
		case *dns.A:
			addr = v.A
		case *dns.AAAA:
			addr = v.AAAA
		default:
			log.Debugf("answer=%+v", answer)
			return p.Default
		}

		if upstream := p.GEO.Match(addr); len(upstream) > 0 {
			log.Debugf("address=%s upstream=%s", addr.String(), upstream)
			return upstream
		}
	}
	return p.Default
}

func (p Policy) Forward(request *dns.Msg) (response *dns.Msg, err error) {
	if remote := p.handleRequestPolicy(request); len(remote) > 0 {
		return upstream.Choose(remote).Forward(request)
	}

	response, err = upstream.Choose(p.Default).Forward(request)
	if err != nil {
		return response, err
	}

	postRemote := p.handleResponsePolicy(response)
	if postRemote == p.Default {
		return response, nil
	}
	return upstream.Choose(postRemote).Forward(request)
}

func parseDomain(request *dns.Msg) string {
	if request == nil || len(request.Question) == 0 {
		log.Debugf("empty request=%s", request.String())
		return ""
	}
	return strings.TrimSuffix(request.Question[0].Name, ".")
}
