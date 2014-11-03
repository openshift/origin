#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

sed -i s/Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers
