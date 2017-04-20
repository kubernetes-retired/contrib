FROM busybox:ubuntu-14.04

ADD ./bin/redis-composite /app/redis-composite-app

WORKDIR /app

CMD ["./redis-composite-app"]

EXPOSE 8082
