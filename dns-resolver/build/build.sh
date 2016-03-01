#!/bin/sh

cd /tmp

# we use an unstable source to build the last version of unbound (1.5.x)
echo "deb-src http://us.archive.ubuntu.com/ubuntu/ xenial main" >> /etc/apt/sources.list

apt-get update && apt-get install -y --no-install-recommends \
  bash \
  file \
  bsdutils \
  openssl \
  dnsutils \
  wget \
  libfstrm-dev \
  libfstrm0 \
  ca-certificates

apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  autoconf \
  autotools-dev \
  bison \
  debhelper \
  dh-autoreconf \
  dh-python \
  flex \
  libevent-dev \
  libexpat1-dev \
  libfstrm-dev \
  libprotobuf-c-dev \
  libssl-dev \
  libtool \
  pkg-config \
  protobuf-c-compiler \
  python-all-dev \
  swig \
  dns-root-data

apt-get source unbound

cd unbound-1.5.7
dpkg-buildpackage

cd ..

dpkg -i \
  unbound-host_1.5.7-1ubuntu1_amd64.deb \
  unbound-anchor_1.5.7-1ubuntu1_amd64.deb \
  unbound_1.5.7-1ubuntu1_amd64.deb \
  python-unbound_1.5.7-1ubuntu1_amd64.deb \
  libunbound2_1.5.7-1ubuntu1_amd64.deb \
