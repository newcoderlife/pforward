package upstream

import (
	"github.com/miekg/dns"
)

type PlainUDPUpstream struct {
	Remote string
}

func (u PlainUDPUpstream) Forward(request *dns.Msg) (*dns.Msg, error) {
	return dns.Exchange(request, u.Remote)
}
