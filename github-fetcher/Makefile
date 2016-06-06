.PHONY: all
all: container

DATE := $(shell date +%F)
GIT := $(shell git rev-parse --verify --short HEAD)

TAG ?= $(DATE)-$(GIT)

DEFAULTREPO := gcr.io/google_containers
REPO ?= $(DEFAULTREPO)
APP := github-fetcher
CONTAINER := $(REPO)/$(APP):$(TAG)

KUBECONFIG ?= $(HOME)/.kube/config
DATABASE ?= localhost

TOKEN ?= "token"
token=$(shell base64 $(TOKEN))

# just build the binary
.PHONY: $(APP)
$(APP):
	GOBIN=$(PWD) CGO_ENABLED=0 GOOS=linux go install -installsuffix cgo -ldflags '-w'

# SQLite requires CGO. It's only used in test.
.PHONY: test
test: $(APP)
	CGO_ENABLED=1 GOOS=linux go test ./...

# build the container with the binary
.PHONY: container
container: $(APP)
	docker build -t $(CONTAINER) .

# push the container
.PHONY: push
push: container
ifeq ($(REPO),$(DEFAULTREPO))
	gcloud docker push $(CONTAINER)
else
	docker push $(CONTAINER)
endif

# updates the deployment.yaml with current build information and sets it to --dry-run
.PHONY: local.deployment.yaml
local.deployment.yaml:
	# update the deployment.yaml with the current date and git hash
	sed -e 's|[[:digit:]]\{4\}-[[:digit:]]\{2\}-[[:digit:]]\{2\}-[[:xdigit:]]\+|$(TAG)|g' deployment.yaml >$@
	# update the deployment.yaml with the current repo (if not gcr.io
	sed -i -e 's|gcr.io/google_containers|$(REPO)|g' $@
	# update the deployment.yaml with database ip
	sed -i -e 's|%DATABASE_IP%|$(DATABASE)|g' $@

.PHONY: local.secret.yaml
local.secret.yaml:
	sed -e 's|1234567890123456789012345678901234567890123456789012345=|$(token)|' secret.yaml >$@

.PHONY: secret
secret: local.secret.yaml
	-kubectl --kubeconfig=$(KUBECONFIG) delete -f $<
	kubectl --kubeconfig=$(KUBECONFIG) create -f $<
	rm $<

# Launch the container on a cluster
# The cluster will need a github oauth token (the secret target makes that easy to create)
.PHONY: deploy
deploy: local.deployment.yaml push secret
	kubectl --kubeconfig=$(KUBECONFIG) apply -f $< --record
	rm $<

# Try to run the binary locally using docker, doesn't need to push or have a running kube cluster.
.PHONY: local_dryrun
local_dryrun: container
	docker run --rm -v $(TOKEN):/token $(CONTAINER) --host $(DATABASE)

.PHONY: clean
clean:
	rm -f $(APP) local.deployment.yaml local.secret.yaml
