# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This Dockerfile will build an image that is configured
# to use Fluentd to collect all Docker container log files
# and then cause them to be ingested using the Google Cloud
# Logging API. This configuration assumes that the host performning
# the collection is a VM that has been created with a logging.write
# scope and that the Logging API has been enabled for the project
# in the Google Developer Console.

FROM gcr.io/google_containers/ubuntu-slim:0.7

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bzip2 \
        ca-certificates \
        libffi-dev \
        libgdbm3 \
        libssl-dev \
        libyaml-dev \
        procps \
        zlib1g-dev \
        libjemalloc1 \
    && rm -rf /var/lib/apt/lists/*

# skip installing gem documentation
RUN mkdir -p /usr/local/etc \
    && { \
        echo 'install: --no-document'; \
        echo 'update: --no-document'; \
    } >> /usr/local/etc/gemrc

ENV RUBY_MAJOR 2.3
ENV RUBY_VERSION 2.3.3
ENV RUBY_DOWNLOAD_SHA256 1a4fa8c2885734ba37b97ffdb4a19b8fba0e8982606db02d936e65bac07419dc
ENV RUBYGEMS_VERSION 2.6.10

ENV BUNDLER_VERSION 1.14.4

# install things globally, for great justice
# and don't create ".bundle" in all our apps
ENV GEM_HOME /usr/local/bundle
ENV BUNDLE_PATH="$GEM_HOME" \
    BUNDLE_BIN="$GEM_HOME/bin" \
    BUNDLE_SILENCE_ROOT_WARNING=1 \
    BUNDLE_APP_CONFIG="$GEM_HOME"
ENV PATH $BUNDLE_BIN:$PATH
RUN mkdir -p "$GEM_HOME" "$BUNDLE_BIN" \
    && chmod 777 "$GEM_HOME" "$BUNDLE_BIN"

# Gemfile with dependencies
COPY Gemfile Gemfile

# some of ruby's build scripts are written in ruby
#   we purge system ruby later to make sure our final image uses what we just built
RUN set -ex \
    \
    && buildDeps=' \
        autoconf \
        bison \
        gcc \
        libbz2-dev \
        libgdbm-dev \
        libglib2.0-dev \
        libncurses-dev \
        libreadline-dev \
        libxml2-dev \
        libxslt-dev \
        make \
        ruby \
        wget \
        xz-utils \
    ' \
    && apt-get update \
    && apt-get install -y --no-install-recommends $buildDeps \
    && rm -rf /var/lib/apt/lists/* \
    \
    && wget -O ruby.tar.xz "https://cache.ruby-lang.org/pub/ruby/${RUBY_MAJOR%-rc}/ruby-$RUBY_VERSION.tar.xz" \
    && echo "$RUBY_DOWNLOAD_SHA256 *ruby.tar.xz" | sha256sum -c - \
    \
    && mkdir -p /usr/src/ruby \
    && tar -xJf ruby.tar.xz -C /usr/src/ruby --strip-components=1 \
    && rm ruby.tar.xz \
    \
    && cd /usr/src/ruby \
    \
# hack in "ENABLE_PATH_CHECK" disabling to suppress:
#   warning: Insecure world writable dir
    && { \
        echo '#define ENABLE_PATH_CHECK 0'; \
        echo; \
        cat file.c; \
    } > file.c.new \
    && mv file.c.new file.c \
    \
    && autoconf \
    && ./configure --disable-install-doc --enable-shared \
    && make -j"$(nproc)" \
    && make install \
    \
    && cd / \
    && rm -r /usr/src/ruby \
    \
    && gem update --system "$RUBYGEMS_VERSION" \
    \
    && gem install bundler --version "$BUNDLER_VERSION" \
    \
    && bundle install \
    \
    && apt-get purge -y --auto-remove $buildDeps

# copy the base Fluentd configuration
COPY fluent.conf /etc/fluent/fluent.conf

# copy the startup script
COPY run.sh /run.sh

CMD /run.sh $FLUENTD_ARGS
