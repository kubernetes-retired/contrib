#! /bin/bash
kubectl exec rd-0 -- redis-cli -h rd-0.redis SET replicated:test true
kubectl exec rd-2 -- redis-cli -h rd-2.redis GET replicated:test

