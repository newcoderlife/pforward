package pforward

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/newcoderlife/pforward/forward"
	"github.com/newcoderlife/pforward/geo"
	"github.com/newcoderlife/pforward/rule"
)

func init() { plugin.Register("pforward", setup) }

func setup(c *caddy.Controller) error {
	inst, err := load(c)
	if err != nil {
		log.Fatalf("load pforward err=%v", err)
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		inst.Next = next
		return inst
	})
	return nil
}

func load(c *caddy.Controller) (*forward.Instance, error) {
	c.Next()

	inst := &forward.Instance{
		Policy:  new(forward.Policy),
		Timeout: time.Second * 3,
	}
	for c.NextBlock() {
		name := c.Val()
		switch name {
		case "default":
			params := c.RemainingArgs()
			if len(params) != 1 {
				return nil, plugin.Error("load", fmt.Errorf("invalid default args=%+v", params))
			}
			inst.Policy.Default = params[0]
		case "ruleset":
			params := c.RemainingArgs()
			if len(params) != 1 {
				return nil, plugin.Error("load", fmt.Errorf("invalid ruleset args=%+v", params))
			}

			files, err := os.ReadDir(params[0])
			if err != nil {
				return nil, plugin.Error("load", fmt.Errorf("invalid ruleset args=%+v err=%v", params, err))
			}

			inst.Policy.Rule = new(rule.Set)
			for _, file := range files {
				if !file.IsDir() && strings.HasSuffix(file.Name(), ".rule") {
					f, err := os.Open(path.Join(params[0], file.Name()))
					if err != nil {
						return nil, plugin.Error("load", fmt.Errorf("invalid rule file=%s err=%v", params[0], err))
					}
					defer f.Close()

					var upstream string
					sc := bufio.NewScanner(f)
					if sc.Scan() {
						first := sc.Text()
						if !strings.HasPrefix(first, "# ") {
							return nil, plugin.Error("load", fmt.Errorf("invalid rule file=%s first=%s", params[0], first))
						}
						upstream = strings.TrimPrefix(first, "# ")
					}

					for sc.Scan() {
						line := sc.Text()
						if err := inst.Policy.Rule.Append(line, upstream); err != nil {
							return nil, plugin.Error("load", err)
						}
					}
				}
			}
		case "geo_database":
			params := c.RemainingArgs()
			if len(params) != 1 {
				return nil, plugin.Error("load", fmt.Errorf("invalid geo_database args=%+v", params))
			}

			if inst.Policy.GEO == nil {
				inst.Policy.GEO = new(geo.GEO)
			}
			if err := inst.Policy.GEO.Open(params[0]); err != nil {
				return nil, plugin.Error("load", fmt.Errorf("invalid geo_database file=%s err=%+v", params[0], err))
			}
		case "geo":
			params := c.RemainingArgs()
			if len(params) != 2 {
				return nil, plugin.Error("load", fmt.Errorf("invalid geo args=%+v", params))
			}

			if inst.Policy.GEO == nil {
				inst.Policy.GEO = new(geo.GEO)
			}
			inst.Policy.GEO.Append(params[0], params[1])
		default:
			log.Debugf("invalid option=%s", name)
		}
	}

	result, _ := json.Marshal(inst)
	log.Infof("instance=%s", string(result))
	return inst, nil
}
