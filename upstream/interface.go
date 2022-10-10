package upstream

import (
	"strings"

	"github.com/miekg/dns"
)

type Upstream interface {
	Forward(request *dns.Msg) (*dns.Msg, error)
}

// ChooseUpstream 先随便糊一个
func Choose(remote string) Upstream {
	if strings.HasPrefix(remote, "https://") {
		return &DOHUpstream{Remote: remote}
	} else {
		return &PlainUDPUpstream{Remote: remote}
	}
}
