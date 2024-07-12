#!/usr/bin/env bash

set -ex

if [ ! -x "/etc/coredns/rules/run.sh" ]; then
  rm -rf /etc/coredns/rules
  git clone https://github.com/newcoderlife/ruleset.git --depth 1 /etc/coredns/rules
fi

ls -ail /etc/coredns/
ls -ail /etc/coredns/rules/

cd /etc/coredns/rules/ && bash run.sh /var/log/coredns.log --update

exec /usr/bin/coredns -conf /etc/coredns/Corefile 2>&1 | tee -a /var/log/coredns.log
