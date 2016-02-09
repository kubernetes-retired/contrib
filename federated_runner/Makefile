test: image
	docker run -v ${HOME}/.aws:/root/.aws -v ${HOME}/.gsutil:/root/.gsutil -v ${HOME}/.config/gcloud:/root/.config/gcloud kubernetes-e2e-runner /runtests.sh

image:
	docker build -t kubernetes-e2e-runner .

