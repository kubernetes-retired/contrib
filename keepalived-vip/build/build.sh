#!/bin/bash

export VERSION=2b600d95582c8ddaa4292d6f8f3d768104a83919

apt-get update && apt-get install -y --no-install-recommends \
  curl \
  gcc \
  libssl-dev \
  libnl-3-dev libnl-route-3-dev libnl-genl-3-dev iptables-dev libnfnetlink-dev libiptcdata0-dev \
  make \
  libipset-dev \
  git \
  libsnmp-dev \
  ca-certificates

cd /tmp

curl -sSL https://github.com/acassen/keepalived/archive/$VERSION.tar.gz | tar xz

cd keepalived-$VERSION
./configure --prefix=/keepalived-1.2.X \
  --sysconfdir=/etc \
  --enable-snmp \
  --enable-sha1

echo "#define GIT_COMMIT \"$VERSION\"" > lib/git-commit.h

make && make install
