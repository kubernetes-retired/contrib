# Kubernetes Contrib

[![Build Status](https://travis-ci.org/kubernetes/contrib.svg)](https://travis-ci.org/kubernetes/contrib)

This is a place for various components in the Kubernetes ecosystem
that aren't part of the Kubernetes core.

## Updating Godeps

The most common dep to update is obviously going to be kuberetes proper. Updating kubernetes, and it's dependancies, can be done as follows:
```
cd $GOPATH/src/github/kubernetes/contrib
godep restore
go get -u github.com/kubernetes/kubernetes
cd $GOPATH/src/github/kubernetes/kubernetes
godep restore
cd $GOPATH/src/github/kubernetes/contrib
rm -rf Godeps
godep save ./...
git [add/remove] as needed
git commit
```

Other deps are similar, although if the dep you wish to update is included from kubernetes we probably want to stay in sync using the above method. If the dep is not in kubernetes proper something like the following should get you a nice clean result:
```
cd $GOPATH/src/github/kubernetes/contrib
godep restore
go get -u $SOME_DEP
rm -rf Godeps
godep save ./...
git [add/remove] as needed
git commit
```
