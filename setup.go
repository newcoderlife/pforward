package pforward

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/dnstap"
	"github.com/coredns/coredns/plugin/pkg/parse"
	"github.com/coredns/coredns/plugin/pkg/proxy"
	pkgtls "github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/miekg/dns"
)

func init() {
	plugin.Register("pforward", setup)
}

func setup(c *caddy.Controller) error {
	fs, err := parseForward(c)
	if err != nil {
		return plugin.Error("pforward", err)
	}
	for i := range fs {
		f := fs[i]
		if f.Len() > max {
			return plugin.Error("pforward", fmt.Errorf("more than %d TOs configured: %d", max, f.Len()))
		}

		if i == len(fs)-1 {
			// last forward: point next to next plugin
			dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
				f.Next = next
				return f
			})
		} else {
			// middle forward: point next to next forward
			nextForward := fs[i+1]
			dnsserver.GetConfig(c).AddPlugin(func(plugin.Handler) plugin.Handler {
				f.Next = nextForward
				return f
			})
		}

		c.OnStartup(func() error {
			return f.OnStartup()
		})
		c.OnStartup(func() error {
			if taph := dnsserver.GetConfig(c).Handler("dnstap"); taph != nil {
				f.SetTapPlugin(taph.(*dnstap.Dnstap))
			}
			return nil
		})

		c.OnShutdown(func() error {
			return f.OnShutdown()
		})
	}

	return nil
}

// OnStartup starts a goroutines for all proxies.
func (f *PForward) OnStartup() (err error) {
	for _, p := range f.proxies {
		p.Start(f.hcInterval)
	}
	return nil
}

// OnShutdown stops all configured proxies.
func (f *PForward) OnShutdown() error {
	for _, p := range f.proxies {
		p.Stop()
	}
	return nil
}

func parseForward(c *caddy.Controller) ([]*PForward, error) {
	var fs = []*PForward{}
	for c.Next() {
		f, err := parseStanza(c)
		if err != nil {
			return nil, err
		}
		fs = append(fs, f)
		log.Infof("Forwarding configured for %+v", Format(f.from.Load()))
	}
	return fs, nil
}

func readRuleset(path string) ([]string, error) {
	dirname := filepath.Dir(path)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path=%s err=%v", path, err)
	}

	zones := make([]string, 0)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		if strings.HasPrefix(line, "include:") {
			subrules, err := readRuleset(filepath.Join(dirname, strings.TrimSpace(strings.TrimPrefix(line, "include:"))))
			if err != nil {
				return nil, fmt.Errorf("unable to read include file '%s': %v", line, err)
			}
			zones = append(zones, subrules...)
		} else {
			zones = append(zones, plugin.Host(line).NormalizeExact()...)
		}
	}

	return zones, nil
}

func parseFrom(c *caddy.Controller, f *PForward) error {
	var (
		from *TrieNode
		path string
	)
	if ok := c.Args(&path); !ok {
		return c.ArgErr()
	}

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		zones, err := readRuleset(path)
		if len(zones) == 0 || err != nil {
			return fmt.Errorf("unable to normalize '%s' '%v'", path, err)
		}

		for _, domain := range zones {
			from = InsertDomain(domain, from)
		}
		f.from.Store(from)

		go func() {
			ticker := time.NewTicker(time.Minute)
			for range ticker.C {
				zones, err := readRuleset(path)
				if len(zones) == 0 || err != nil {
					log.Errorf("update domains err=%v", err)
					continue
				}

				for _, domain := range zones {
					from = InsertDomain(domain, from)
				}
				f.from.Store(from)
				log.Infof("update domains=%s", Format(f.from.Load()))
			}
		}()

		return nil
	}

	zones := plugin.Host(path).NormalizeExact()
	if len(zones) == 0 {
		return fmt.Errorf("unable to normalize '%s'", path)
	}

	for _, domain := range zones {
		from = InsertDomain(domain, from)
	}
	f.from.Store(from)

	return nil
}

func parseStanza(c *caddy.Controller) (*PForward, error) {
	f := New()

	if err := parseFrom(c, f); err != nil {
		return f, err
	}

	to := c.RemainingArgs()
	if len(to) == 0 {
		return f, c.ArgErr()
	}

	toHosts, err := parse.HostPortOrFile(to...)
	if err != nil {
		return f, err
	}

	transports := make([]string, len(toHosts))
	allowedTrans := map[string]bool{"dns": true, "tls": true}
	for i, host := range toHosts {
		trans, h := parse.Transport(host)

		if !allowedTrans[trans] {
			return f, fmt.Errorf("'%s' is not supported as a destination protocol in forward: %s", trans, host)
		}
		p := proxy.NewProxy("forward", h, trans)
		f.proxies = append(f.proxies, p)
		transports[i] = trans
	}

	for c.NextBlock() {
		if err := parseBlock(c, f); err != nil {
			return f, err
		}
	}

	if f.tlsServerName != "" {
		f.tlsConfig.ServerName = f.tlsServerName
	}

	// Initialize ClientSessionCache in tls.Config. This may speed up a TLS handshake
	// in upcoming connections to the same TLS server.
	f.tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(len(f.proxies))

	for i := range f.proxies {
		// Only set this for proxies that need it.
		if transports[i] == transport.TLS {
			f.proxies[i].SetTLSConfig(f.tlsConfig)
		}
		f.proxies[i].SetExpire(f.expire)
		f.proxies[i].GetHealthchecker().SetRecursionDesired(f.opts.HCRecursionDesired)
		// when TLS is used, checks are set to tcp-tls
		if f.opts.ForceTCP && transports[i] != transport.TLS {
			f.proxies[i].GetHealthchecker().SetTCPTransport()
		}
		f.proxies[i].GetHealthchecker().SetDomain(f.opts.HCDomain)
	}

	return f, nil
}

func parseBlock(c *caddy.Controller, f *PForward) error {
	config := dnsserver.GetConfig(c)
	switch c.Val() {
	case "max_fails":
		if !c.NextArg() {
			return c.ArgErr()
		}
		n, err := strconv.ParseUint(c.Val(), 10, 32)
		if err != nil {
			return err
		}
		f.maxfails = uint32(n)
	case "health_check":
		if !c.NextArg() {
			return c.ArgErr()
		}
		dur, err := time.ParseDuration(c.Val())
		if err != nil {
			return err
		}
		if dur < 0 {
			return fmt.Errorf("health_check can't be negative: %d", dur)
		}
		f.hcInterval = dur
		f.opts.HCDomain = "."

		for c.NextArg() {
			switch hcOpts := c.Val(); hcOpts {
			case "no_rec":
				f.opts.HCRecursionDesired = false
			case "domain":
				if !c.NextArg() {
					return c.ArgErr()
				}
				hcDomain := c.Val()
				if _, ok := dns.IsDomainName(hcDomain); !ok {
					return fmt.Errorf("health_check: invalid domain name %s", hcDomain)
				}
				f.opts.HCDomain = plugin.Name(hcDomain).Normalize()
			default:
				return fmt.Errorf("health_check: unknown option %s", hcOpts)
			}
		}

	case "force_tcp":
		if c.NextArg() {
			return c.ArgErr()
		}
		f.opts.ForceTCP = true
	case "prefer_udp":
		if c.NextArg() {
			return c.ArgErr()
		}
		f.opts.PreferUDP = true
	case "tls":
		args := c.RemainingArgs()
		if len(args) > 3 {
			return c.ArgErr()
		}

		for i := range args {
			if !filepath.IsAbs(args[i]) && config.Root != "" {
				args[i] = filepath.Join(config.Root, args[i])
			}
		}
		tlsConfig, err := pkgtls.NewTLSConfigFromArgs(args...)
		if err != nil {
			return err
		}
		f.tlsConfig = tlsConfig
	case "tls_servername":
		if !c.NextArg() {
			return c.ArgErr()
		}
		f.tlsServerName = c.Val()
	case "expire":
		if !c.NextArg() {
			return c.ArgErr()
		}
		dur, err := time.ParseDuration(c.Val())
		if err != nil {
			return err
		}
		if dur < 0 {
			return fmt.Errorf("expire can't be negative: %s", dur)
		}
		f.expire = dur
	case "policy":
		if !c.NextArg() {
			return c.ArgErr()
		}
		switch x := c.Val(); x {
		case "random":
			f.p = &random{}
		case "round_robin":
			f.p = &roundRobin{}
		case "sequential":
			f.p = &sequential{}
		default:
			return c.Errf("unknown policy '%s'", x)
		}
	case "max_concurrent":
		if !c.NextArg() {
			return c.ArgErr()
		}
		n, err := strconv.Atoi(c.Val())
		if err != nil {
			return err
		}
		if n < 0 {
			return fmt.Errorf("max_concurrent can't be negative: %d", n)
		}
		f.ErrLimitExceeded = errors.New("concurrent queries exceeded maximum " + c.Val())
		f.maxConcurrent = int64(n)
	case "backup_request":
		if !c.NextArg() {
			return c.ArgErr()
		}
		backupDuration, err := time.ParseDuration(c.Val())
		if err != nil {
			return c.ArgErr()
		}
		f.backupDuration = backupDuration
		/* // 暂不支持 threshold
		threshold := 0.1
		if c.NextArg() {
			threshold, err = strconv.ParseFloat(c.Val(), 64)
			if err != nil {
				return c.ArgErr()
			}
		}
		*/

	default:
		return c.Errf("unknown property '%s'", c.Val())
	}

	return nil
}

const max = 15 // Maximum number of upstreams.
