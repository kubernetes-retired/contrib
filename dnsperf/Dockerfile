FROM ubuntu
MAINTAINER Abhishek Shah "abshah@google.com"

RUN apt-get -q update

#Install dependencies needed for building dnsperf
RUN apt-get install -y build-essential
RUN apt-get install -y bind9utils libbind-dev libkrb5-dev libssl-dev libcap-dev libxml2-dev libgeoip-dev

#Install curl, wget is not required, but adding it in for convenience
RUN apt-get install -y curl wget

#Fetch dnsperf
RUN curl ftp://ftp.nominum.com/pub/nominum/dnsperf/2.0.0.0/dnsperf-src-2.0.0.0-1.tar.gz -O
RUN tar xfvz dnsperf-src-2.0.0.0-1.tar.gz

#Get query data file for dnsperf
RUN wget ftp://ftp.nominum.com/pub/nominum/dnsperf/data/queryfile-example-current.gz
COPY queryfile-local-services queryfile-local-services
RUN gunzip queryfile-example-current.gz
RUN cat queryfile-example-current | head -10000 > queryfile-example-short

#Build dnsperf
RUN cd /dnsperf-src-2.0.0.0-1 && ./configure
RUN cd /dnsperf-src-2.0.0.0-1 && make clean
RUN cd /dnsperf-src-2.0.0.0-1 && make
RUN cd /dnsperf-src-2.0.0.0-1 && make install

#Install dnsutils
RUN apt-get install -y dnsutils

COPY runjob.sh /runjob.sh

ENTRYPOINT [ "/runjob.sh" ]
