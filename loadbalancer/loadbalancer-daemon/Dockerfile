FROM gcr.io/google_containers/ubuntu-slim:0.3
RUN apt-get update
RUN apt-get install -y --no-install-recommends \
  nginx \
  gettext-base \
  ca-certificates \
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
  net-tools \
  iputils-ping \
  curl && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/keepalived && \
  ln -s /keepalived/sbin/keepalived /usr/sbin && \
  ln -s /keepalived/bin/genhash /usr/sbin

# forward nginx access and error logs to stdout and stderr of the daemon
# controller process
RUN ln -sf /proc/1/fd/1 /var/log/nginx/access.log \
	&& ln -sf /proc/1/fd/2 /var/log/nginx/error.log

COPY loadbalancer-daemon /
COPY backend/backends/nginx/nginx.tmpl /

ENTRYPOINT ["/loadbalancer-daemon"]
