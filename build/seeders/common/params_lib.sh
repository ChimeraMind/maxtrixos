#!/bin/bash
set -eu

# The default dating scheme is "YYYYMMDD" anchored to the first past Monday.
params_lib.get_chroot_date() {
    date -d "$(( $(date +%u) - 1 )) days ago" +%Y%m%d
}