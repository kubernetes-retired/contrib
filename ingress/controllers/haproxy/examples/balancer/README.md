# HAProxy Ingress Controller Pod

This example creates a Pod (default restart policy is `Always`) tied to one node. This should be easily converted to DaemonSet or Deployment object.

- Stackpoint.io will keep the stable HAProxy ingress controller at `quay.io/stackpoint/haproxy-ingress-controller:<version>`
- You need to open firewall ports for the balancer node
- This example is using an image built from this repo
- Namespace, Image, Versions, API Port, External IP, should be customized for your environment
