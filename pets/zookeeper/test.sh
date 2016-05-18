#! /bin/bash
kubectl exec zoo-0 -- bin/zkCli.sh create /foo bar;
kubectl exec zoo-2 -- bin/zkCli.sh get /foo;

