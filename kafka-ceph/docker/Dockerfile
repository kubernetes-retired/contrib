# Kafka:2.11_0.9.0
# yeepay/kafka:2.11-0.9.0
FROM ubuntu:14.04
MAINTAINER dawei.li@yeepay.com 

# Add User
RUN mkdir -p /home/kafka

WORKDIR /home/kafka

# Install Kafka
RUN sudo apt-get update -yqq && sudo apt-get install -y default-jre wget

RUN wget https://www.apache.org/dyn/closer.cgi?path=/kafka/0.9.0.1/kafka_2.11-0.9.0.1.tgz

RUN tar -zxvf /home/kafka/kafka_2.11-0.9.0.1.tgz

EXPOSE 9092
