FROM alpine:edge

ENV GIT_SYNC_DEST /git

RUN apk add -U bash git && \
  rm -rf /var/cache/apk/*

VOLUME ["/git"]

COPY git-sync /git-sync

RUN touch /.netrc && chmod 777 /.netrc

ENTRYPOINT ["/git-sync"]
