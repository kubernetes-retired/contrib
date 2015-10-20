all:

TAG = 0.0
PREFIX = bprashanth/nginxhttps

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	if [ -f *.key ]; then rm -f *.key; fi
	if [ -f *.crt ]; then rm -f *.crt; fi
	if [ -f *.json ]; then rm -f *.json; fi
