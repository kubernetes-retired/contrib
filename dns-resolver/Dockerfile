FROM gcr.io/google_containers/ubuntu-slim:0.1

ENV DEBIAN_FRONTEND noninteractive

COPY unbound-host_1.5.7-1ubuntu1_amd64.deb /tmp/unbound-host_1.5.7-1ubuntu1_amd64.deb
COPY unbound-anchor_1.5.7-1ubuntu1_amd64.deb /tmp/unbound-anchor_1.5.7-1ubuntu1_amd64.deb
COPY unbound_1.5.7-1ubuntu1_amd64.deb /tmp/unbound_1.5.7-1ubuntu1_amd64.deb
COPY python-unbound_1.5.7-1ubuntu1_amd64.deb /tmp/python-unbound_1.5.7-1ubuntu1_amd64.deb
COPY libunbound2_1.5.7-1ubuntu1_amd64.deb /tmp/libunbound2_1.5.7-1ubuntu1_amd64.deb

RUN apt-get update && apt-get install -y \
  nsd3 \
  libexpat1 \
  dns-root-data \
  libevent-2.0-5 \
  libprotobuf-c1 \
  libpython2.7 \
  python \
  openssl \
  curl \
  libfstrm0 && \
  curl -sSL -o /tmp/nsd_4.1.7-1_amd64.deb http://mirrors.kernel.org/ubuntu/pool/universe/n/nsd/nsd_4.1.7-1_amd64.deb && \
  dpkg -i /tmp/*.deb && rm /tmp/*.deb && \
  apt-get clean -y && \
  rm -rf /var/lib/apt/lists/*

COPY rootfs /

ENTRYPOINT ["/kube2dns"]
