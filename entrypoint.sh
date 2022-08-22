#!/bin/sh

cp -R /tmp/.ssh /root/.ssh 2>/dev/null
chmod 700 /root/.ssh 2>/dev/null
chmod 600 /root/.ssh/* 2>/dev/null
chmod 644 /root/.ssh/*.pub 2>/dev/null

hetzner-k3s "$@"
