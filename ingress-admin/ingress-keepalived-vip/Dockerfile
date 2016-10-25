FROM index.caicloud.io/caicloud/keepalived:v1.2.19

# Set the default timezone to Shanghai
RUN echo "Asia/Shanghai" > /etc/timezone
RUN dpkg-reconfigure -f noninteractive tzdata

RUN apt-get update && apt-get install -y --no-install-recommends \
  libssl1.0.0 \
  libnl-3-200 \
  libnl-route-3-200 \
  libnl-genl-3-200 \
  iptables \
  libnfnetlink0 \
  libiptcdata0 \
  libipset3 \
  libipset-dev \
  libsnmp30 \
  kmod \
  ca-certificates \
  iproute2 \
  ipvsadm \
  bash && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

COPY ingress-keepalived-vip /
COPY keepalived.tmpl /
COPY keepalived.conf /etc/keepalived

CMD ["./ingress-keepalived-vip"]
