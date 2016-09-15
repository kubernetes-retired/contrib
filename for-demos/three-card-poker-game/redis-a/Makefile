REGISTRY?=suyogbarve
release: clean build push clean

build:
	docker build --rm --force-rm -t ${REGISTRY}/redis-a-builder .
	docker run --rm ${REGISTRY}/redis-a-builder | docker build -t ${REGISTRY}/redis-a-app:latest -

push:
	docker push ${REGISTRY}/redis-a-app:latest

clean:
	docker rm -f ${REGISTRY}/redis-a-builder 2 > /dev/null || true
	docker rmi -f ${REGISTRY}/redis-a-builder || true
	docker rmi -f ${REGISTRY}/redis-a-app:latest || true 

