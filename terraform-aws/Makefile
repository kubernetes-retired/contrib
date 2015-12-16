all: tokens.tfvars

kube-up: tokens.tfvars
	terraform get
	terraform apply -var-file=tokens.tfvars
	terraform output "kubectl configuration" > cfg_kubectl.sh
	bash -x cfg_kubectl.sh

kube-down:
	terraform destroy -var-file=tokens.tfvars

tokens.tfvars: gen_tokens.sh
	bash ./gen_tokens.sh > $@

graph:
	terraform graph | dot -Tpng > graph.png

clean:
	rm tokens.tfvars

.PHONY: graph clean
