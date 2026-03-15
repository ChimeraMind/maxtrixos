#!/bin/bash
set -eu

if [ -z "${__MATRIXOS_ENV_PARSED:-}" ]; then

source "${MATRIXOS_DEV_DIR}"/lib/env_lib.sh

# matrixOS Git repos URLs and paths.
MATRIXOS_OVERLAY_GIT_REPO=$(env_lib.get_simple_var "matrixOS" "OverlayGitRepo")
MATRIXOS_PRIVATE_EXAMPLE_GIT_REPO=$(env_lib.get_simple_var "matrixOS" "PrivateExampleGitRepo")
MATRIXOS_PRIVATE_GIT_REPO_PATH=$(env_lib.get_root_var "${MATRIXOS_DEV_DIR}" "matrixOS" "PrivateGitRepoPath")
MATRIXOS_DEFAULT_PRIVATE_GIT_REPO_PATH=$(env_lib.get_root_var "${MATRIXOS_DEV_DIR}" "matrixOS" "DefaultPrivateGitRepoPath")

__MATRIXOS_ENV_PARSED=1
fi