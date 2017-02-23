# Overview

This directory contains docker images with fluentd, which can be later
configured to collect and ingest logs.

# Usage

Each image is built with its own set of plugins which you can later use
in the configuration. The set of plugin is enumerated in a Gemfile in the
image's directory. You can find details about fluentd configuration on the
[official site](http://docs.fluentd.org/articles/config-file).

In order to configure fluentd image, you should mount directory with `.conf`
files to `/etc/fluent/config.d` or add files to that directory by building
a new image on top. All `.conf` files in the `/etc/fluent/config.d` directory
will be included to the final fluentd configuration.

Command line arguments to the fluentd executable are passed
via environment variable `FLUENTD_ARGS`.