# dns-resolver
==============

This module is a replacement of the [dns addon](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/dns). The principal difference is the use of [unbound](https://www.unbound.net) as caching DNS resolver that forwards queries for the kubernetes domain to an in-memory dns server and an external DNS server for queries not located in the kubernetes domain.

This is the diagram
```
53    tcp
         \  
          unbound -> queries domain cluster kubernetes    -> in-memory dns server (miekg/dns based)
         /         \_ forward external queries  8.8.8.8
53    udp

8081  tcp healthz and dns content dump (/dump)
```
