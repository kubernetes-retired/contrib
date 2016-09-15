REGISTRY?=suyogbarve
release: clean build push clean

build:
	docker build --rm --force-rm -t ${REGISTRY}/demo-client-builder .
	docker run --rm ${REGISTRY}/demo-client-builder | docker build -t ${REGISTRY}/demo-client-app:latest -

push:
	docker push ${REGISTRY}/demo-client-app:latest

clean:
	docker rm -f ${REGISTRY}/demo-client-builder 2 > /dev/null || true
	docker rmi -f ${REGISTRY}/demo-client-builder || true
	docker rmi -f ${REGISTRY}/demo-client-app:latest || true 

