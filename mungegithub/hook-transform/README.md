Hook-transform is a Kubernetes deployement used to receive Github
webhooks. It changes the format and pushes it inside a Pub/Sub queue so
that they can be later processed by the mungebot.

Deploying, updating configmap:
```
# Create container and push it to google-containers
make push

# Make sure you update the config version
kubectl create configmap webhook-config-v10 --from-file=config.yaml

# Edit deployment with new container name and/or config version

kubectl apply -f deployment.yaml
```
