#!/bin/bash
set -eu

if [ -z "${__MATRIXOS_ENV_PARSED:-}" ]; then

# The "special var".
DEFAULT_MATRIXOS_DEV_DIR=/matrixos
MATRIXOS_DEV_DIR=${MATRIXOS_DEV_DIR:-"${DEFAULT_MATRIXOS_DEV_DIR}"}

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh

# Re-initialize MATRIXOS_DEV_DIR from config.
MATRIXOS_DEV_DIR=$(env_lib.get_root "${MATRIXOS_DEV_DIR}")
MATRIXOS_ARTIFACTS_DIR=$(env_lib.get_artifactsroot "${MATRIXOS_DEV_DIR}")

# matrixOS Git repos URLs and paths.
MATRIXOS_OVERLAY_GIT_REPO=$(env_lib.get_simple_var "matrixOS" "OverlayGitRepo")
MATRIXOS_PRIVATE_EXAMPLE_GIT_REPO=$(env_lib.get_simple_var "matrixOS" "PrivateExampleGitRepo")
MATRIXOS_PRIVATE_GIT_REPO_PATH=$(env_lib.get_root_var "${MATRIXOS_DEV_DIR}" "matrixOS" "PrivateGitRepoPath")
MATRIXOS_DEFAULT_PRIVATE_GIT_REPO_PATH=$(env_lib.get_root_var "${MATRIXOS_DEV_DIR}" "matrixOS" "DefaultPrivateGitRepoPath")

## Seeders section
MATRIXOS_SEEDER_GPG_KEYS_DIR=$(env_lib.get_root_var "${MATRIXOS_ARTIFACTS_DIR}" "Seeder" "GpgKeysDir")

__MATRIXOS_ENV_PARSED=1
fi