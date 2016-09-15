FROM golang:1.6.0

ADD . /go/src/github.io/suyogbarve/kubernetes/demo-client/src

WORKDIR /go/src/github.io/suyogbarve/kubernetes/demo-client

RUN cd src/ && go get && go build -o ../bin/demo-client

RUN cp src/app/Dockerfile .

CMD tar cvzf - .
