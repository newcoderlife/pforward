#!/usr/bin/env bash

cd /etc/coredns/rules/ && ./run.sh /var/log/coredns.log --update && cd /root/
cat /etc/coredns/rules/local.noncn

/usr/bin/coredns -conf /etc/coredns/Corefile 2>&1 | tee -a /var/log/coredns.log

exit 0
