

# Pull base image.
FROM openjdk:alpine 

MAINTAINER Suyog Barve<suyogbarve@gmail.com>

COPY ./target/three-card-poker-0.1.0.jar /data/

# Define working directory.
WORKDIR /data

EXPOSE 8080
# Define default command.
#CMD ["/usr/bin/java -jar /data/three-card-poker-0.1.0.jar"]
ENTRYPOINT ["java", "-Dserver.port=80",  "-jar", "/data/three-card-poker-0.1.0.jar"]
