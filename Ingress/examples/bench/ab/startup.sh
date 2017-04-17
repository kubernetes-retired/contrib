#! /bin/bash

# Run cron in the background
crontab /etc/cron.d/build_index
cron -f &

# Build the index once do we don't have to wait a minute for cron
/build_index.sh /siteroot/fs

# Run nginx in the foreground
nginx -g "daemon off;"
