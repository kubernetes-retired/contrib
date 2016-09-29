FROM haproxy:1.6-alpine
MAINTAINER Matt Baldwin (baldwin@stackpointcloud.com)


ENV BALANCER_DIR /opt/stackpoint/balancer
ENV PATH $BALANCER_DIR/bin/:$PATH
ENV BALANCER_CFG $BALANCER_DIR/cfg/haproxy.cfg
ENV BALANCER_API_PORT 8207

RUN mkdir -p $BALANCER_DIR/cfg $BALANCER_DIR/bin $BALANCER_DIR/scripts $BALACNER_DIR/certs

WORKDIR $BALANCER_DIR/bin/

COPY ./files/haproxy.cfg $BALANCER_CFG
COPY ./bin/haddock $BALANCER_DIR/bin/
COPY ./scripts/haproxy.sh $BALANCER_DIR/scripts/
COPY ./scripts/docker-entrypoint.sh /


ENTRYPOINT ["/docker-entrypoint.sh"]
