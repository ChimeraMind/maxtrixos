#!/bin/bash
# bedrock.sh (hook) - Execute a customization script for QA or other CI/CD systems
#                    to consume right before committing the root filesystem (CHROOT_DIR)
#                    to the local ostree repository. This hook should return non-zero exit
#                    status in case of issues. Warnings must be logged to stderr.
#
# These are the env variables that are made available:
#
#
# CHROOT_DIR=/path/to/chroot
# The directory path to a the root filesystem ready to be committed to ostree.
#
# MATRIXOS_DEV_DIR=/path/to/matrixos-toolkit-dir (e.g. /matrixos)
# The directory path to the matrixos-toolkit repository. This is useful for sourcing
# common functions and utilities.
#
# MATRIXOS_PRIVATE_GIT_REPO_PATH=/path/to/private-git-repo
# The directory path to the private git repository. This directory is expected to
# be already empty at this stage.
#
set -e

source "${MATRIXOS_DEV_DIR}/release/hooks/matrixos/amd64/common.sh"

source "${MATRIXOS_DEV_DIR}/release/hooks/matrixos/amd64/common.sh"


setup_networkd() {
    local imagedir="${1}"
    echo "Setting up systemd-networkd configuration for DHCP in ${imagedir}"

    local networkd_dir="${imagedir}/etc/systemd/network"
    mkdir -p "${networkd_dir}"
    cat > "${networkd_dir}/20-matrixos-wired.network" << EOF
[Match]
Type=ether

[Network]
DHCP=yes
EOF
}

main() {
    local funcs=(
        setup_networkd
        release_common.check_nvidia_module
        release_common.check_ryzen_smu_module
        release_common.list_top_packages
        release_common.check_leaking_buckets
    )
    local exit_code=0
    for func in "${funcs[@]}"; do
        if ! "${func}" "${CHROOT_DIR}"; then
            echo "${func} failed, exiting with error." >&2
            exit_code=1
        else
            echo "${func} completed successfully."
        fi
    done
    if [ "${exit_code}" != "0" ]; then
        return "${exit_code}"
    fi
}

main "${@}"

