# Policy Forward

## Info

A CoreDNS plugin for domain and GEO policy forward. Fork from official forward plugin.

## Config

```
# Default Corefile, see https://coredns.io for more information.
# For PForward configuration, see https://github.com/newcoderlife/pforward for help.

(snip) {
    cache
    metadata

    log `{type} {name} {/pforward/upstream} {duration} {/pforward/response/ip}`
}

.:60001 {
    import snip

    pforward . tls://1.1.1.1 tls://1.0.0.1 {
       tls_servername cloudflare-dns.com
       health_check 5s
    }
}

.:60002 {
    import snip

    pforward . tls://8.8.8.8 tls://8.8.4.4 {
        tls_servername dns.google
        health_check 5s
    }
}

.:60003 {
    import snip

    pforward . tls://1.12.12.12 tls://120.53.53.53 {
        tls_servername dot.pub
        health_check 5s
    }
}

.:60004 {
    import snip

    pforward . tls://223.5.5.5 tls://223.6.6.6 {
        tls_servername dns.alidns.com
        health_check 5s
    }
}

.:53 {
    bind 0.0.0.0

    pforward /etc/coredns/rules/ruleset.noncn 127.0.0.1:60001 127.0.0.1:60002
    pforward . 127.0.0.1:60003 127.0.0.1:60004

    template ANY AAAA {
        rcode NXDOMAIN
    }

    cache
    errors
}
```

## Release

You can build from source. Require golang latest version from [official](https://go.dev/dl/).

Or download debian package for linux64 from [Releases](https://github.com/newcoderlife/pforward/releases).