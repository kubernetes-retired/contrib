### Build Kafka Docker images
----------------------------------------

##### Build 

```bash
docker build -t kafka:2.11-0.9.0 .
```

```bash
docker tag kafka:2.11-0.9.0  registry.docker:5000/yeepay/kafka:2.11-0.9.0 
docker push registry.docker:5000/yeepay/kafka:2.11-0.9.0 
```
