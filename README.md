# Policy Forward

## Info

A CoreDNS plugin for domain and GEO policy forward.

## Config

```
. {
    bind 0.0.0.0

    pforward {
        default https://1.1.1.1/dns-query

        ruleset /etc/coredns/rules/

        geo_database /etc/coredns/GeoLite2-Country.mmdb
        geo cn https://1.12.12.12/dns-query
    }

    template ANY AAAA {
        rcode NXDOMAIN
    }

    cache {
        success 65536
    }
    errors
}
```

## Release

You can build from source. Require golang latest version from [official](https://go.dev/dl/).

Or download debian package for linux64 from [Releases](https://github.com/newcoderlife/pforward/releases).
