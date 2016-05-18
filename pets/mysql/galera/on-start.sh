#! /bin/bash
/on-change.sh
# TODO: make this mysqld_safe? ideally we should just add an upstart, systemd
# or health check instead.
mysqld &
