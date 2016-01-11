#!/bin/bash
sleep 2
netctl net create -s 20.1.1.0/24 -g 20.1.1.254 -p 1001 k8s-poc
netctl net create -s 20.1.2.0/24 -g 20.1.2.254 -p 1002 k8s-default
netctl group create k8s-poc poc-epg
netctl group create k8s-poc default-epg
