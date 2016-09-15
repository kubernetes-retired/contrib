REGISTRY?=suyogbarve
release: clean build push clean

build:
	docker build --rm --force-rm -t ${REGISTRY}/redis-composite-builder .
	docker run --rm ${REGISTRY}/redis-composite-builder | docker build -t ${REGISTRY}/redis-composite-app:latest -

push:
	docker push ${REGISTRY}/redis-composite-app:latest

clean:
	docker rm -f ${REGISTRY}/redis-composite-builder 2 > /dev/null || true
	docker rmi -f ${REGISTRY}/redis-composite-builder || true
	docker rmi -f ${REGISTRY}/redis-composite-app:latest || true 

