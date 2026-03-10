#!/bin/bash
# This file is sourced inside preppers prepper.sh (outside chroot) scripts.
# It contains common prepper execution variables.
set -eu

if [ -z "${__MATRIXOS_PREPPERS_ENV_PARSED:-}" ]; then

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh

# See conf/matrixos.conf for documentation on these variables.
MATRIXOS_PREPPERS_USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC=$(env_lib.get_bool_var "Seeder" "UseCpReflinkModeInsteadOfRsync")

__MATRIXOS_PREPPERS_ENV_PARSED=1
fi