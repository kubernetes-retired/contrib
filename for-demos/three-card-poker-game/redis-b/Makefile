REGISTRY?=suyogbarve
release: clean build push clean

build:
	docker build --rm --force-rm -t ${REGISTRY}/redis-b-builder .
	docker run --rm ${REGISTRY}/redis-b-builder | docker build -t ${REGISTRY}/redis-b-app:latest -

push:
	docker push ${REGISTRY}/redis-b-app:latest

clean:
	docker rm -f ${REGISTRY}/redis-b-builder 2 > /dev/null || true
	docker rmi -f ${REGISTRY}/redis-b-builder || true
	docker rmi -f ${REGISTRY}/redis-b-app:latest || true 

