# PolicyForward

## Info

A CoreDNS plugin for domain policy forward.

## Config

```
.:53 {
    pforward {
        114.114.114.114                           // default using 114.114.114.114:53
        policy /etc/coredns/google.policy 1.1.1.1 // domain in policy file using 1.1.1.1:53

        auto 1.1.1.1                              // judge response GEO and make correct answer
        geo /etc/coredns/GeoLite2-Country.mmdb.db // maxmind datebase location
        block_ipv6                                // block all AAAA response
    }
}
```

## Release

You can build from source. Require golang latest version from [official](https://go.dev/dl/).

Or download linux_amd64 binary and debian package from [Github Actions](https://github.com/newcoderlife/pforward/actions).
