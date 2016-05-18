#! /bin/bash
kubectl exec mysql-0 -- mysql -u root -e "create database test;"
kubectl exec mysql-1 -- mysql -u root -e "use test; create table pet (id int(10), name varchar(20));"
kubectl exec mysql-1 -- mysql -u root -e "use test; insert into pet (id, name) values (1, \"galera\");"
kubectl exec mysql-2 -- mysql -u root -e "use test; select * from pet;"

