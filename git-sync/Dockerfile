FROM ubuntu:15.10

ENV GIT_SYNC_DEST /git
VOLUME ["/git"]

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && \
  apt-get install -y git ca-certificates --no-install-recommends && \
  apt-get clean -y && \
  rm -rf /var/lib/apt/lists/*

COPY git-sync /git-sync

RUN mkdir /nonexistent && chmod 777 /nonexistent

ENTRYPOINT ["/git-sync"]
