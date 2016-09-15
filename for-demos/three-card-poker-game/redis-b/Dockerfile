FROM golang:1.6.0

ADD . /go/src/github.io/suyogbarve/kubernetes/poker/src

WORKDIR /go/src/github.io/suyogbarve/kubernetes/poker

RUN cd src/ && go get && go build -o ../bin/redis-b

RUN cp src/app/Dockerfile .

CMD tar cvzf - .
