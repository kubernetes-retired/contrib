# Kubernetes Contrib

[![Build Status](https://travis-ci.org/kubernetes/contrib.svg)](https://travis-ci.org/kubernetes/contrib)

This is a place for various components in the Kubernetes ecosystem
that aren't part of the Kubernetes core.

## Updating Godeps

Godeps in contrib/ has a different layout than in kubernetes/ proper. This is because
contrib contains multiple tiny projects, each with their own dependencies. Each
in contrib/ has it's own Godeps.json. For example the Godeps.json for Ingress
is Ingress/Godeps/Godeps.json. This means that godeps commands like `godep restore`
or `godep test` work in the root directory. Theys should be run from inside the
subproject directory you want to test

## Updating Godeps

The most common dep to update is obviously going to be kuberetes proper. Updating
kubernetes and it's dependancies in the Ingress subproject for example can be done
as follows:
```
cd $GOPATH/src/github.com/kubernetes/contrib/Ingress
godep restore
go get -u github.com/kubernetes/kubernetes
cd $GOPATH/src/github.com/kubernetes/kubernetes
godep restore
cd $GOPATH/src/github/kubernetes/contrib/Ingress
rm -rf Godeps
godep save ./...
git [add/remove] as needed
git commit
```

Other deps are similar, although if the dep you wish to update is included from
kubernetes we probably want to stay in sync using the above method. If the dep is not in kubernetes proper something like the following should get you a nice clean result:
```
cd $GOPATH/src/github/kubernetes/contrib/Ingress
godep restore
go get -u $SOME_DEP
rm -rf Godeps
godep save ./...
git [add/remove] as needed
git commit
```

## Running all tests

To run all go test in all projects do this:
```
./hack/for-go-proj.sh test
```
