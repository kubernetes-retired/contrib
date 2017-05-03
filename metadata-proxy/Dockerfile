FROM gcr.io/google-containers/debian-base-amd64:0.1
LABEL maintainer "qlee@google.com"

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
  apt-utils && apt-get install -y \
  iptables \
  nginx \
  sudo

# Give nginx its config file.
COPY nginx.conf /etc/nginx/nginx.conf
# Place our wrapper script into the image.
COPY start-proxy.sh /

ENTRYPOINT ["/start-proxy.sh"]
