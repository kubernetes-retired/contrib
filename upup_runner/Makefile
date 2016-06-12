test: image
	docker run -v ${HOME}/.ssh/id_rsa.pub:/root/.ssh/id_rsa.pub -v ${HOME}/.aws:/root/.aws -v ${HOME}/.gsutil:/root/.gsutil -v ${HOME}/.config/gcloud:/root/.config/gcloud kubernetes-e2e-runner-upup /runtests.sh kubernetes-e2e-upup-aws-ha

image:
	docker build -t kubernetes-e2e-runner-upup .

