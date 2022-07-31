# PolicyForward

## Info

A CoreDNS plugin for domain policy forward.

## Config

```
.:53 {
    pforward {
        114.114.114.114                           // default using 114.114.114.114:53
        policy /etc/coredns/google.policy 1.1.1.1 // domain in policy file using 1.1.1.1:53
    }
}
```
