FROM gcr.io/google_containers/ubuntu-slim:0.1

RUN echo "deb http://us.archive.ubuntu.com/ubuntu/ xenial main universe" >> /etc/apt/sources.list && \
  apt-get update && apt-get install -y --no-install-recommends unbound && \
  apt-get clean -y && \
  rm -rf /var/lib/apt/lists/*

COPY rootfs /

ENTRYPOINT ["/kube2dns"]
