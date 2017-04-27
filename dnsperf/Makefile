TAG:=1.0
REGISTRY:=gcr.io/google.com/gkalele-netperf
NAME:= dnsperf

push:
	docker build -t $(REGISTRY)/$(NAME):$(TAG) .
	gcloud docker push $(REGISTRY)/$(NAME):$(TAG)

runtests:
	kubectl create -f dnsperf-job.yaml
