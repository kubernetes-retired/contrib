#! /bin/bash
kubectl exec cs-0 -- cqlsh -e "create keyspace foo with replication = {'class': 'SimpleStrategy', 'replication_factor': 3};"
kubectl exec cs-2 -- cqlsh -e "describe keyspace foo;"

