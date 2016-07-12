FROM gcr.io/google_containers/ubuntu-slim:0.3
RUN apt-get update
RUN apt-get install -y --no-install-recommends \
  nginx \
  keepalived && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# forward nginx access and error logs to stdout and stderr of the daemon
# controller process
RUN ln -sf /proc/1/fd/1 /var/log/nginx/access.log \
	&& ln -sf /proc/1/fd/2 /var/log/nginx/error.log

COPY loadbalancer-daemon /
COPY backend/backends/nginx/nginx.tmpl /
COPY keepalived/keepalived.tmpl /

ENTRYPOINT ["/loadbalancer-daemon"]
