# Policy Forward

## Info

A CoreDNS plugin for domain and GEO policy forward. Fork from official forward plugin.

## Feature

- Support multiple zones with [ruleset](https://github.com/newcoderlife/ruleset).
- Support backup request. See [Retry](https://www.cloudwego.io/docs/kitex/tutorials/service-governance/retry/).

## Config

```
# Default Corefile, see https://coredns.io for more information.
# For PForward configuration, see https://github.com/newcoderlife/pforward for help.

.:60001 {
    pforward . tls://1.1.1.1 tls://1.0.0.1 {
       tls_servername cloudflare-dns.com
       policy round_robin
       backup_request 2s
       expire 3.5s
    }
}

.:60002 {
    pforward . tls://8.8.8.8 tls://8.8.4.4 {
        tls_servername dns.google
        policy round_robin
        backup_request 2s
        expire 3.5s
    }
}

.:60003 {
    pforward . tls://1.12.12.12 tls://120.53.53.53 {
        tls_servername dot.pub
        policy round_robin
    }
}

.:60004 {
    pforward . tls://223.5.5.5 tls://223.6.6.6 {
        tls_servername dns.alidns.com
        policy round_robin
    }
}

.:53 {
    bind 0.0.0.0

    cache {
        success 65536 1440 360
        prefetch 10 60m
    }
    metadata

    pforward /etc/coredns/rules/noncn 127.0.0.1:60001 127.0.0.1:60002 {
        policy round_robin
        backup_request 3s
        expire 5s
    }
    pforward . 127.0.0.1:60003 127.0.0.1:60004 {
        policy round_robin
    }

    template ANY AAAA {
        rcode NOERROR
    }

    log . "{type} {name} {/pforward/upstream} {/pforward/backup} {duration} {/pforward/response/ip}"
    errors
}
```

## Release

You can build from source. Require golang latest version from [official](https://go.dev/dl/).

Or download debian package for linux64 from [Releases](https://github.com/newcoderlife/pforward/releases).