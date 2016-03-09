# Nginx Ingress Controller

This is a simple nginx Ingress controller. Expect it to grow up. See [Ingress controller documentation](../README.md) for details on how it works.

This version includes SSL support, integrated with HashiCorp Vault and access and error logs, which go to the container stdout/stderr by default.

Access to Vault is predicated on the following:

Vault is running in the local environment.

A valid access token is available at the following location:

/etc/vault-token/ingress-read-only

An example for creating this as a kubernetes secret is shown below.

apiVersion: v1
kind: Secret
metadata:
  namespace: kube-system
  name: ingress-token
data:
  ingress-read-only: %%TOKEN%%

where %%TOKEN%% is an access token for a policy with read access to secret/ssl in Vault:

# For Ingress controller- ssl key access
path "sys/*" {
  policy = "deny"
}

path "secret/ssl/*" {
  policy = "read"
}

The key contents themselves should already have been written to Vault as follows:

Key: secrets/ssl/<hostname> crt and key

where "crt" contains the x509 public certificate and key contains the x509 private key.

vault write www.example.com key="-----BEGIN PRIVATE KEY-----..." crt="-----BEGIN CERTIFICATE-----..."

## Deploying the controller

Deploying the controller is as easy as creating the RC in this directory. Having done so you can test it with the following echoheaders application:

```yaml
apiVersion: v1
kind: ReplicationController
metadata:
  namespace: kube-system
  name: nginx-ingress
  labels:
    app: nginx-ingress
spec:
  replicas: 3
  selector:
    app: nginx-ingress
  template:
    metadata:
      labels:
        app: nginx-ingress
    spec:
      containers:
      - image: devlm/nginx-ingress:dev
        imagePullPolicy: Always
        name: nginx-ingress
        env:
          - name: "VAULT_ADDR"
            value: "http://vault.kube-system.svc.cluster.local:8243"
          - name: "VAULT_SKIP_VERIFY"
            value: "false"
        ports:
        - containerPort: 80
          hostPort: 80
        - containerPort: 443
          hostPort: 443
        volumeMounts:
          - name: vault-volume
            mountPath: /etc/vault-token
      volumes:
        - name: vault-volume
          secret:
              secretName: ingress-token
      nodeSelector:
        role: loadbalancer
```

Note the secret volume setup and `VAULT_` environment variables.

## Deploying an ingress:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: example
  namespace: some-namespace
  labels:
    ssl: true
spec:
  rules:
  - host: www.example.com
    http:
      paths:
      - backend:
          serviceName: example
          servicePort: 8080
        path: /
```

Note the `ssl: true` label.

The ingress controller will detect that this has been created and react as follows:

```
Found secret for www.example.com
Found key for www.example.com
Found crt for www.example.com
Starting nginx [-c /etc/nginx/nginx.conf]
nginx config updated.
```

You should now be able to point www.example.com at the Ingress nodes and reach www.example.com over http:// and https://.

If either VAULT_ADDR or VAULT_TOKEN are not set then vault support is disabled and the controller should act like nginx-alpha, with the addition of access and err logging from the nginx instance to stdout/err on the container.