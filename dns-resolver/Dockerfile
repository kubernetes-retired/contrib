FROM gcr.io/google_containers/debian-slim:0.1

COPY libunbound2_1.5.7-2_amd64.deb /tmp/libunbound2_1.5.7-2_amd64.deb
COPY unbound-anchor_1.5.7-2_amd64.deb /tmp/unbound-anchor_1.5.7-2_amd64.deb
COPY unbound-host_1.5.7-2_amd64.deb /tmp/unbound-host_1.5.7-2_amd64.deb
COPY unbound_1.5.7-2_amd64.deb /tmp/unbound_1.5.7-2_amd64.deb

RUN apt-get update && apt-get install -y \
  wget \
  nsd3 \
  libexpat1 \
  dns-root-data \
  libevent-2.0-5 \
  libprotobuf-c1 \
  libpython2.7 && \
  wget http://http.us.debian.org/debian/pool/main/f/fstrm/libfstrm0_0.2.0-1_amd64.deb && \
  wget http://http.us.debian.org/debian/pool/main/n/nsd/nsd_4.1.7-1~bpo80+1_amd64.deb && \
  dpkg -i *.deb && rm *.deb && \
  dpkg -i /tmp/*.deb && rm /tmp/*.deb && \
  apt-get clean -y && \
  rm -rf /var/lib/apt/lists/*

COPY rootfs /

ENTRYPOINT ["/kube2dns"]
