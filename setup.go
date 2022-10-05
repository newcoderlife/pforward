package pforward

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/oschwald/geoip2-golang"
)

func init() { plugin.Register("pforward", setup) }

func setup(c *caddy.Controller) error {
	pForward, err := load(c)
	if err != nil {
		log.Fatalf("[setup] load err=%v", err)
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		pForward.Next = next
		return pForward
	})

	return nil
}

func load(c *caddy.Controller) (*PForward, error) {
	c.Next()

	p := &PForward{
		Policy: MakePolicy(),
	}
	for c.NextBlock() {
		name := c.Val()
		switch name {
		case "policy": // policy policy_file nameserver
			params := c.RemainingArgs()
			if len(params) != 2 {
				return nil, errors.New("invalid policy config")
			}

			f, err := os.Open(params[0])
			if err != nil || f == nil {
				return nil, errors.New("invalid policy file")
			}
			defer f.Close()

			sc := bufio.NewScanner(f)
			for sc.Scan() {
				p.Policy.AddRule(sc.Text(), params[1])
			}

			log.Infof("[load] file=%s", params[0])
		case "auto":
			params := c.RemainingArgs()
			if len(params) != 2 {
				return nil, plugin.Error("pforward", fmt.Errorf("invalid auto args=%+v", params))
			}

			p.AutoCNServer, p.AutoAbroadServer = params[0], params[1]

			log.Infof("[load] auto %s %s", p.AutoCNServer, p.AutoAbroadServer)
		case "geo":
			params := c.RemainingArgs()
			if len(params) != 1 {
				return nil, plugin.Error("pforward", fmt.Errorf("invalid geo args=%+v", params))
			}

			var err error
			p.GeoDatabase, err = geoip2.Open(params[0])
			if err != nil {
				return nil, plugin.Error("pforward", fmt.Errorf("invalid geo database err=%+v", err))
			}

			log.Infof("[load] load geoip")
		case "block_ipv6":
			p.BlockAAAA = true

			log.Infof("[load] block_ipv6")
		case "timeout":
			params := c.RemainingArgs()
			if len(params) != 1 {
				return nil, plugin.Error("pforward", fmt.Errorf("invalid timeout args=%+v", params))
			}

			t, err := strconv.Atoi(params[0])
			if err != nil {
				return nil, plugin.Error("pforward", fmt.Errorf("invalid timeout err=%+v", err))
			}

			p.Timeout = time.Millisecond * time.Duration(t)
			log.Infof("[load] timeout=%dmilli", t)
		default:
			p.Policy.AddRule(".", name)
		}
	}

	log.Infof("[load] rule count=%d", p.Policy.Count())
	return p, nil
}
