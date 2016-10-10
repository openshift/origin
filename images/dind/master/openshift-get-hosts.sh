#!/bin/sh
grep '\-node' /etc/hosts | awk '{print $2}'
