#!/bin/bash
set -eu

if [ -z "${__MATRIXOS_ENV_PARSED:-}" ]; then

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh

__MATRIXOS_ENV_PARSED=1
fi