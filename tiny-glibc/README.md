### debian-iptables

Serves as the base image for binaries that may not be compiled statically.
This image includes `glibc`, but is based on `busybox`, so it's just ~11MB!

This image is compiled for multiple architectures.

#### How to release

If you're editing the Dockerfile or some other thing, please bump the `TAG` in the Makefile.

```console
# Build for linux/amd64 (default)
$ make push ARCH=amd64
# ---> gcr.io/google_containers/tiny-glibc-amd64:TAG

$ make push ARCH=arm
# ---> gcr.io/google_containers/tiny-glibc-arm:TAG

$ make push ARCH=arm64
# ---> gcr.io/google_containers/tiny-glibc-arm64:TAG

$ make push ARCH=ppc64le
# ---> gcr.io/google_containers/tiny-glibc-ppc64le:TAG
```

If you don't want to push the images, run `make` or `make build` instead
