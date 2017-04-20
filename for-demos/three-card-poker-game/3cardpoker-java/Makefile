REGISTRY?=suyogbarve
MAVEN?=/opt/apache-maven-3.3.9/bin/mvn
release: clean build push clean

build:
	${MAVEN} clean install
	docker build -t ${REGISTRY}/three-card-poker-java:latest .

push:
	docker push ${REGISTRY}/three-card-poker-java:latest

clean:
	docker rmi -f ${REGISTRY}/three-card-poker-java:latest || true
	rm -fr ./target