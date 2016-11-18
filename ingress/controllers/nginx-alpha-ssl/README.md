# Nginx Ingress Controller

This is a simple nginx Ingress controller. Expect it to grow up. See [Ingress controller documentation](../README.md) for details on how it works.

This version includes SSL support, integrated with HashiCorp Vault and access and error logs, which go to the container stdout/stderr by default.

## Installation

```
go build controller.go && docker rmi devlm/nginx-ingress:latest; docker build -t devlm/nginx-ingress:latest . && docker push devlm/nginx-ingress:latest
```

replacing devlm/nginx-ingress with a repo of your choice. There is a pre-built container at devlm/nginx-ingress on docker hub if required.

Access to Vault is predicated on the following:

Vault is running in the local environment.

A valid access token is available at the following location:

/etc/vault-token/ingress-read-only

An example for creating this as a kubernetes secret is shown below.

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: kube-system
  name: ingress-read-only
data:
  ingress-read-only: %%TOKEN%%
```

where %%TOKEN%% is an access token for a policy with read access to secret/ssl in Vault:

```yaml
# For Ingress controller- ssl key access
path "sys/*" {
  policy = "deny"
}

path "secret/ssl/*" {
  policy = "read"
}
```
You may also pass in an alternative file as an env var $VAULT_TOKEN_FILE, or pass in the TOKEN directly as $VAULT_TOKEN. The controller self-renews this token every 10 minutes.

The key contents themselves should already have been written to Vault as follows:

Key: secrets/ssl/<hostname> crt and key

where "crt" contains the x509 public certificate and key contains the x509 private key.

```
vault write www.example.com key="-----BEGIN PRIVATE KEY-----..." crt="-----BEGIN CERTIFICATE-----..."
```

## Deploying the controller

Deploying the controller is as easy as creating the RC in this directory. Having done so you can test it with the following echoheaders application:

```yaml
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  namespace: kube-system
  name: nginx-ingress
spec:
 template:
    metadata:
      labels:
        name: nginx-ingress
    spec:
      hostNetwork: true
      nodeSelector:
        role: loadbalancer
      containers:
      - image: devlm/nginx-ingress:1.7.0
        resources:
            limits:
              cpu: "${INGRESS_CPU}"
              memory: ${INGRESS_RAM}
        imagePullPolicy: Always
        name: nginx-ingress
        env:
          - name: "VAULT_ADDR"
            value: "https://vault.kube-system.svc.cluster.local:8243"
          - name: "VAULT_TOKEN_FILE"
            value: "/etc/vault-token/ingress-read-only"
          - name: "VAULT_SSL_SIGNER"
            value: >
                   "-----BEGIN CERTIFICATE-----
                    ....
                    -----END CERTIFICATE-----"
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
              secretName: ingress-read-only

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
    httpsOnly: false
    httpsBackend: true
spec:
  rules:
  - host: www.example.com
    http:
      paths:
      - backend:
          serviceName: example
          servicePort: 8043
        path: /
```
You should be able to access the Services on the public IP of the node the nginx pod lands on.

Note the `ssl: true` label.

The other (optional) labels available are:

`httpsOnly` Do not create a http listener
`httpsBackend` Use https for the proxy connection to the backend
`timeout` A configurable override of the backend timeout (default 60s)

The ingress controller will detect that this has been created and react as follows:

```
Found secret for www.example.com
Found key for www.example.com
Found crt for www.example.com
Keypair correct for www.example.com
Service address responding for www.example.com
Starting nginx [-c /etc/nginx/nginx.conf]
nginx config updated.
```
You should now be able to point www.example.com at the Ingress nodes and reach www.example.com over http:// and https://.

Validation:

The ingress will NOT be created if:
The keypair created by the key and crt values is not valid.
The backend service does not respond at https://example.some-namespace.svc.cluster.local:8043 with a 200.

However it will keep trying.

If either VAULT_ADDR or VAULT_TOKEN are not set then vault support is disabled and the controller should act like nginx-alpha, with the addition of access and err logging from the nginx instance to stdout/err on the container.
