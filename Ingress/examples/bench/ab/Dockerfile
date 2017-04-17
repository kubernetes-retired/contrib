# Copyright 2015 The Kubernetes Authors. All rights reserved.
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

FROM ubuntu:14.04
MAINTAINER Prashanth B <beeps@google.com>

ENV    TERM xterm
ENV    DEBIAN_FRONTEND noninteractive
RUN    echo "deb-src http://archive.ubuntu.com/ubuntu trusty main" >> /etc/apt/sources.list
RUN    sed 's/main$/main universe/' -i /etc/apt/sources.list

# Debugging
RUN apt-get -y update; apt-get -y install curl vim wget

# Openresty nginx
RUN    apt-get -y build-dep nginx \
  && apt-get -q -y clean && rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/*
RUN    wget http://openresty.org/download/ngx_openresty-1.7.10.1.tar.gz \
  && tar xvfz ngx_openresty-1.7.10.1.tar.gz \
  && cd ngx_openresty-1.7.10.1 \
  && ./configure --with-luajit --sbin-path=/usr/sbin --prefix=/usr/local \
  && make \
  && make install \
  && rm -rf /ngx_openresty*

# /stress
RUN wget http://people.seas.harvard.edu/~apw/stress/stress-1.0.4.tar.gz \
  && tar xvfz stress-1.0.4.tar.gz  \
  && cd stress-1.0.4 \
  && ./configure \
  && make \
  && make install \
  && rm -rf /stress*

# /fs static files
COPY build_index.sh /
COPY index.html /siteroot/index.html
COPY nginx.conf /usr/local/nginx/conf/nginx.conf
COPY nginx.html /siteroot/fs/files/nginx.html
COPY nginx.jpg /siteroot/fs/images/nginx.jpg

# /fs index cron
COPY build_index.cron /etc/cron.d/build_index
RUN chmod 0644 /etc/cron.d/build_index
RUN touch /var/log/cron.log
RUN crontab /etc/cron.d/build_index
RUN cron

# Bootstrap
COPY startup.sh /
RUN chown -R www-data:www-data /siteroot; chmod -R 755 /siteroot

ENTRYPOINT ["/startup.sh"]
