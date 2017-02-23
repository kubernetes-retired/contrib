# Overview

This directory contains docker images with fluentd, which can be later configured to collect
log from any source.

# Usage

Each image is built with its own set of plugins which you can later use in configuration. Set of
plugin is enumerated in Gemfile. You can find details about fluentd configuration on the
[official site](http://docs.fluentd.org/articles/config-file).

In order to configure fluentd image, you should mount directory with `.conf` files to
`/etc/fluent/config` or add files to this directory by building a new image on top. All `.conf`
files will be included to the final fluentd configuration.

Command line arguments to the fluentd can be passed via environment variable `FLUENTD_ARGS`.