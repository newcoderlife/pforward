package forward

import (
	"context"
	"time"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
)

var (
	log = clog.NewWithPlugin("pforward")
)

type Instance struct {
	Next    plugin.Handler
	Policy  *Policy
	Timeout time.Duration
}

func (Instance) Name() string {
	return "pforward"
}

func (i Instance) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	response, err := i.Policy.Forward(r)
	if err != nil {
		log.Debugf("request=%s err=%v", r.String(), err)
		return plugin.NextOrFailure(i.Name(), i.Next, ctx, w, r)
	}

	if err := w.WriteMsg(response); err != nil {
		log.Debugf("request=%s err=%v", r.String(), err)
		return plugin.NextOrFailure(i.Name(), i.Next, ctx, w, r)
	}

	if i.Next != nil {
		return i.Next.ServeDNS(ctx, w, r)
	}
	return dns.RcodeSuccess, nil
}
