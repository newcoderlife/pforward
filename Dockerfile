FROM debian:stable-slim

RUN apt-get -qq update && apt-get -yyqq upgrade && apt-get -yyqq install ca-certificates libcap2-bin python3 python3-venv jq git wget curl && apt-get clean

ARG TARGETARCH
COPY coredns_${TARGETARCH}.deb /root/
COPY entry.sh /root/
RUN dpkg -i /root/*.deb && rm -rf /root/*.deb
RUN setcap cap_net_bind_service=+ep /usr/bin/coredns

VOLUME [ "/var/log/", "/etc/coredns/" ]
EXPOSE 53/udp

ENTRYPOINT ["/root/entry.sh"]
