REGISTRY?=suyogbarve
release: clean build push clean

build:
	docker build -t ${REGISTRY}/three-card-poker:latest .

push:
	docker push ${REGISTRY}/three-card-poker:latest

clean:
	docker rmi -f ${REGISTRY}/three-card-poker:latest || true