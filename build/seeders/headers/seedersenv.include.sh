#!/bin/bash
# This file is sourced inside seeders chroot.sh (inside chroot) scripts.
# It contains common seeder execution variables.
set -eu

if [ -z "${__MATRIXOS_SEEDERS_ENV_PARSED:-}" ]; then

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh


__MATRIXOS_SEEDERS_ENV_PARSED=1
fi