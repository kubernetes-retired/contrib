FROM alpine:3.3

COPY unbound.apk /var/cache/apk/unbound.apk

RUN apk add -U \
  bash \
  file \
  nsd \
  openssl \
  bind-tools && \
  apk add --allow-untrusted /var/cache/apk/unbound.apk && \
  rm -rf /var/cache/apk/* /tmp/* /root/.cpan /usr/share/man

COPY rootfs /

ENTRYPOINT ["/kube2dns"]
