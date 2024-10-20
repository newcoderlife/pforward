// Package pforward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just opening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package pforward

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/debug"
	"github.com/coredns/coredns/plugin/dnstap"
	"github.com/coredns/coredns/plugin/metadata"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	otext "github.com/opentracing/opentracing-go/ext"
)

var log = clog.NewWithPlugin("forward")

const (
	defaultExpire = 10 * time.Second
	hcInterval    = 500 * time.Millisecond
)

// PForward represents a plugin instance that can proxy requests to another (DNS) server. It has a list
// of proxies each representing one upstream proxy.
type PForward struct {
	concurrent int64 // atomic counters need to be first in struct for proper alignment

	proxies    []*proxy.Proxy
	p          Policy
	hcInterval time.Duration

	from atomic.Pointer[TrieNode]

	tlsConfig     *tls.Config
	tlsServerName string
	maxfails      uint32
	expire        time.Duration
	maxConcurrent int64

	backupDuration time.Duration // duration=0: disabled

	opts proxy.Options // also here for testing

	// ErrLimitExceeded indicates that a query was rejected because the number of concurrent queries has exceeded
	// the maximum allowed (maxConcurrent)
	ErrLimitExceeded error

	tapPlugins []*dnstap.Dnstap // when dnstap plugins are loaded, we use to this to send messages out.

	Next plugin.Handler
}

// New returns a new Forward.
func New() *PForward {
	f := &PForward{maxfails: 2, tlsConfig: new(tls.Config), expire: defaultExpire, p: new(random), hcInterval: hcInterval, opts: proxy.Options{ForceTCP: false, PreferUDP: false, HCRecursionDesired: true, HCDomain: "."}}
	return f
}

// SetProxy appends p to the proxy list and starts healthchecking.
func (f *PForward) SetProxy(p *proxy.Proxy) {
	f.proxies = append(f.proxies, p)
	p.Start(f.hcInterval)
}

// SetTapPlugin appends one or more dnstap plugins to the tap plugin list.
func (f *PForward) SetTapPlugin(tapPlugin *dnstap.Dnstap) {
	f.tapPlugins = append(f.tapPlugins, tapPlugin)
	if nextPlugin, ok := tapPlugin.Next.(*dnstap.Dnstap); ok {
		f.SetTapPlugin(nextPlugin)
	}
}

// Len returns the number of configured proxies.
func (f *PForward) Len() int { return len(f.proxies) }

// Name implements plugin.Handler.
func (f *PForward) Name() string { return "forward" }

type TaskResult struct {
	Result *dns.Msg
	Err    error
	Backup bool
}

func (f *PForward) ConnectWithTimeout(ctx context.Context, state request.Request, proxies []*proxy.Proxy, opts proxy.Options) (*dns.Msg, error) {
	if len(proxies) == 0 {
		return nil, ErrNoForward
	}
	if f.backupDuration == 0 || len(proxies) == 1 {
		return proxies[0].Connect(ctx, state, opts)
	}

	results := make(chan *TaskResult, len(proxies))
	ctx, cancel := context.WithTimeout(ctx, f.backupDuration)
	defer cancel()

	go func() { // first request
		ret, err := proxies[0].Connect(ctx, state, opts)
		results <- &TaskResult{Result: ret, Err: err}
	}()

	go func() { // backup request
		select {
		case <-ctx.Done():
		case <-time.After(f.backupDuration):
			ret, err := proxies[1].Connect(ctx, state, opts)
			results <- &TaskResult{Result: ret, Err: err, Backup: true}
		}
	}()

	count := 0
	for result := range results {
		if result != nil && result.Err == nil {
			metadata.SetValueFunc(ctx, "pforward/backup", func() string {
				return strconv.FormatBool(result.Backup)
			})
			return result.Result, nil
		}
		if count++; count == 2 {
			break
		}
	}

	return nil, fmt.Errorf("all upstreams failed")
}

// ServeDNS implements plugin.Handler. // TODO need refactoring
func (f *PForward) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	if !f.match(state) {
		return plugin.NextOrFailure(f.Name(), f.Next, ctx, w, r)
	}

	if f.maxConcurrent > 0 {
		count := atomic.AddInt64(&(f.concurrent), 1)
		defer atomic.AddInt64(&(f.concurrent), -1)
		if count > f.maxConcurrent {
			maxConcurrentRejectCount.Add(1)
			return dns.RcodeRefused, f.ErrLimitExceeded
		}
	}

	fails := 0
	var span, child ot.Span
	var upstreamErr error
	span = ot.SpanFromContext(ctx)
	i := 0
	list := f.List()
	deadline := time.Now().Add(defaultTimeout)
	start := time.Now()
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if i >= len(list) {
			// reached the end of list, reset to begin
			i = 0
			fails = 0
		}

		proxy := list[i]
		currentProxies := list[i:]
		i++
		if proxy.Down(f.maxfails) {
			fails++
			if fails < len(f.proxies) {
				continue
			}
			// All upstream proxies are dead, assume healthcheck is completely broken and randomly
			// select an upstream to connect to.
			r := new(random)
			proxy = r.List(f.proxies)[0]

			healthcheckBrokenCount.Add(1)
		}

		if span != nil {
			child = span.Tracer().StartSpan("connect", ot.ChildOf(span.Context()))
			otext.PeerAddress.Set(child, proxy.Addr())
			ctx = ot.ContextWithSpan(ctx, child)
		}

		metadata.SetValueFunc(ctx, "pforward/upstream", func() string {
			return proxy.Addr()
		})

		var (
			ret *dns.Msg
			err error
		)
		opts := f.opts

		for {
			ret, err = f.ConnectWithTimeout(ctx, state, currentProxies, opts)

			if err == ErrCachedClosed { // Remote side closed conn, can only happen with TCP.
				continue
			}
			// Retry with TCP if truncated and prefer_udp configured.
			if ret != nil && ret.Truncated && !opts.ForceTCP && opts.PreferUDP {
				opts.ForceTCP = true
				continue
			}
			break
		}

		if child != nil {
			child.Finish()
		}

		if len(f.tapPlugins) != 0 {
			toDnstap(ctx, f, proxy.Addr(), state, opts, ret, start)
		}

		upstreamErr = err

		if err != nil {
			// Kick off health check to see if *our* upstream is broken.
			if f.maxfails != 0 {
				proxy.Healthcheck()
			}

			if fails < len(f.proxies) {
				continue
			}
			break
		}

		// Check if the reply is correct; if not return FormErr.
		if !state.Match(ret) {
			debug.Hexdumpf(ret, "Wrong reply for id: %d, %s %d", ret.Id, state.QName(), state.QType())

			formerr := new(dns.Msg)
			formerr.SetRcode(state.Req, dns.RcodeFormatError)
			w.WriteMsg(formerr)
			return 0, nil
		}

		metadata.SetValueFunc(ctx, "pforward/response/ip", func() string {
			if ret == nil || len(ret.Answer) == 0 {
				return "-"
			}

			for _, ans := range ret.Answer {
				switch ans.Header().Header().Rrtype {
				case dns.TypeA:
					return ans.(*dns.A).A.String()
				case dns.TypeAAAA:
					return ans.(*dns.AAAA).AAAA.String()
				}
			}
			return "-"
		})

		w.WriteMsg(ret)
		return 0, nil
	}

	if upstreamErr != nil {
		return dns.RcodeServerFailure, upstreamErr
	}

	return dns.RcodeServerFailure, ErrNoHealthy
}

func (f *PForward) match(state request.Request) bool {
	return FindDomainSuffix(state.Name(), f.from.Load())
}

// ForceTCP returns if TCP is forced to be used even when the request comes in over UDP.
func (f *PForward) ForceTCP() bool { return f.opts.ForceTCP }

// PreferUDP returns if UDP is preferred to be used even when the request comes in over TCP.
func (f *PForward) PreferUDP() bool { return f.opts.PreferUDP }

// List returns a set of proxies to be used for this client depending on the policy in f.
func (f *PForward) List() []*proxy.Proxy { return f.p.List(f.proxies) }

var (
	// ErrNoHealthy means no healthy proxies left.
	ErrNoHealthy = errors.New("no healthy proxies")
	// ErrNoForward means no forwarder defined.
	ErrNoForward = errors.New("no forwarder defined")
	// ErrCachedClosed means cached connection was closed by peer.
	ErrCachedClosed = errors.New("cached connection was closed by peer")
)

// Options holds various Options that can be set.
type Options struct {
	// ForceTCP use TCP protocol for upstream DNS request. Has precedence over PreferUDP flag
	ForceTCP bool
	// PreferUDP use UDP protocol for upstream DNS request.
	PreferUDP bool
	// HCRecursionDesired sets recursion desired flag for Proxy healthcheck requests
	HCRecursionDesired bool
	// HCDomain sets domain for Proxy healthcheck requests
	HCDomain string
}

var defaultTimeout = 5 * time.Second
