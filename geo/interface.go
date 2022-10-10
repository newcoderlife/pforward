package geo

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type rule struct {
	Region   string
	Upstream string
}

type GEO struct {
	database *geoip2.Reader
	rules    []*rule
}

func (g GEO) Match(ip net.IP) string {
	ctry, err := g.database.Country(ip)
	if err != nil || ctry == nil {
		return ""
	}

	for _, r := range g.rules {
		if r.Region == ctry.Country.IsoCode {
			return r.Upstream
		}
	}
	return ""
}

func (g *GEO) Open(path string) (err error) {
	g.database, err = geoip2.Open(path)
	return err
}

func (g *GEO) Append(region, upstream string) {
	g.rules = append(g.rules, &rule{Upstream: upstream, Region: strings.ToUpper(region)})
}
