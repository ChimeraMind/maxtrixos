#!/bin/bash
# This file is sourced inside preppers prepper.sh (outside chroot) scripts.
# It contains common prepper execution variables.
set -eu

if [ -z "${__MATRIXOS_PREPPERS_ENV_PARSED:-}" ]; then

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh

__MATRIXOS_PREPPERS_ENV_PARSED=1
fi