FROM debian:stable-slim
ARG TARGETARCH

RUN sed -i 's/deb.debian.org/mirrors.aliyun.com/g' /etc/apt/sources.list.d/debian.sources
RUN apt-get -qq update && apt-get -yyqq upgrade && apt-get -yyqq install ca-certificates libcap2-bin python3 python3-venv jq git wget curl && apt-get clean
COPY coredns_${TARGETARCH}.deb /root/
RUN dpkg -i /root/*.deb && rm -rf /root/*.deb
RUN setcap cap_net_bind_service=+ep /usr/bin/coredns

VOLUME [ "/var/log/", "/etc/coredns/" ]
EXPOSE 53/udp

COPY entry.sh /root/
RUN chmod +x /root/entry.sh

ENTRYPOINT [ "/bin/bash", "-c", "/root/entry.sh" ]