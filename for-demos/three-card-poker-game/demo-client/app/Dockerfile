FROM busybox:ubuntu-14.04

ADD ./bin/demo-client /app/demo-client
ADD ./src/static /app/static

WORKDIR /app

CMD ["./demo-client"]

EXPOSE 8083
