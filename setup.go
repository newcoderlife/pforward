package pforward

import (
	"bufio"
	"errors"
	"os"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() { plugin.Register("pforward", setup) }

func setup(c *caddy.Controller) error {
	policy, err := load(c)
	if err != nil {
		log.Fatalf("[setup] load err=%v", err)
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return MakePForward(next, policy)
	})

	return nil
}

func load(c *caddy.Controller) (*Policy, error) {
	c.Next()

	p := MakePolicy()
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
				p.AddRule(sc.Text(), params[1])
			}

			log.Infof("[load] file=%s", params[0])
		default:
			p.AddRule(".", name)
		}
	}

	log.Infof("[load] rule count=%d", p.Count())
	return p, nil
}
