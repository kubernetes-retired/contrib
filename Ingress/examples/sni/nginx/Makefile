all:

TAG = 0.0
PREFIX = bprashanth/nginxrouter

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm -f *.crt
	rm -f *.key
	rm -f *.json
