#!/bin/bash

get_src()
{
  hash="$1"
  url="$2"
  f=$(basename "$url")

  curl -sSL "$url" -o "$f"
  echo "$hash  $f" | sha256sum -c - || exit 10
  tar xzf "$f"
  rm -rf "$f"
}

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

# download, verify and extract the source files
get_src $SHA256 \
  "https://github.com/acassen/keepalived/archive/v$VERSION.tar.gz"

cd keepalived-$VERSION
./configure --prefix=/keepalived \
  --sysconfdir=/etc \
  --enable-snmp \
  --enable-sha1

make && make install

tar -czvf /keepalived.tar.gz /keepalived
