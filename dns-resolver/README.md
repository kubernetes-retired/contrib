# dns-resolver
==============

This module is a replacement of the [dns addon](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns). The principal difference is the use of [unbound](https://www.unbound.net) as caching DNS resolver that forwards queries for the kubernetes domain to skydns and uses an external DNS server for other queries (like in skydns).

This is the diagram
```
53    tcp
         \  
          unbound -> queries domain cluster kubernetes    -> skydns
         /         \_ forward external queries  8.8.8.8
53    udp

8080  tcp healthz
```
