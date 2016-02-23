#!/bin/sh

cd /tmp

# we use an unstable source to build the last version of unbound (1.5.x)
echo "deb-src http://ftp.us.debian.org/debian sid main" >> /etc/apt/sources.list

apt-get update && apt-get install -y --no-install-recommends \
  bash \
  file \
  bsdutils \
  nsd \
  openssl \
  dnsutils \
  wget \
  ca-certificates

wget http://ftp.us.debian.org/debian/pool/main/f/fstrm/libfstrm-dev_0.2.0-1_amd64.deb
wget http://ftp.us.debian.org/debian/pool/main/f/fstrm/libfstrm0_0.2.0-1_amd64.deb
dpkg -i *.deb

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

dpkg -i libunbound2_1.5.7-2_amd64.deb unbound-anchor_1.5.7-2_amd64.deb unbound-host_1.5.7-2_amd64.deb unbound_1.5.7-2_amd64.deb
