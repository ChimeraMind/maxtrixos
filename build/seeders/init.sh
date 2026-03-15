#!/bin/bash
# This script is used to prepare the chroot environment for a seed and then
# execute the chroot() syscall to start the seeding process.
# This script is effectively the entrypoint for unshare and acts as PID 1, until
# the exec call at the end (which makes the target seeding script PID 1).
#
# The env vars passed to this script are:
# - Those mentioned in build/seeder/*/chroot.sh.
# - Additional, env vars mentioned below.
#
# Additional env vars:
#
# SEEDER_PRIVATE_GIT_REPO_PATH
# The path to the private git repository from outside the chroot. This is expected
# to be bind mount inside the chroot at DEFAULT_PRIVATE_GIT_REPO_PATH.
#
# SEEDER_DISTFILES_DIR
# The path to the distfiles directory from outside the chroot. This is expected
# to be bind mount inside the chroot at /var/cache/distfiles if not already
# mounted.
# 
# SEEDER_BINPKGS_DIR
# The path to the binpkgs directory from outside the chroot. This is expected
# to be bind mount inside the chroot at /var/cache/binpkgs if not already
# mounted.
#
# This script takes the following arguments:
# $1: The path to the chroot dir.
# $2: The path to the seeding script to execute inside the chroot.
# $3..$n: The arguments to pass to the seeding script.
set -eu


setup_chroot_env() {
    local chroot_dir="${1}"

    # Mount the necessary filesystems for the chroot environment.
    local dst_distfiles="${chroot_dir%/}/var/cache/distfiles"
    mkdir -p "${SEEDER_DISTFILES_DIR}"
    mkdir -p "${dst_distfiles}"

    if ! mountpoint -q "${dst_distfiles}"; then
        echo "Mounting ${dst_distfiles} ..."
        mount --bind "${SEEDER_DISTFILES_DIR}" "${dst_distfiles}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_distfiles}"
    else
        echo "${dst_distfiles} already mounted, skipping bind mount."
    fi

    local dst_binpkgs="${chroot_dir%/}/var/cache/binpkgs"
    mkdir -p "${SEEDER_BINPKGS_DIR}"
    mkdir -p "${dst_binpkgs}"

    if ! mountpoint -q "${dst_binpkgs}"; then
        echo "Mounting ${dst_binpkgs} ..."
        mount --bind "${SEEDER_BINPKGS_DIR}" "${dst_binpkgs}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_binpkgs}"
    else
        echo "${dst_binpkgs} already mounted, skipping bind mount."
    fi

    local dst_private_git="${chroot_dir%/}/${DEFAULT_PRIVATE_GIT_REPO_PATH#/}"
    mkdir -p "${dst_private_git}"
    if ! mountpoint -q "${dst_private_git}"; then
        echo "Mounting ${dst_private_git} ..."
        mount --bind "${SEEDER_PRIVATE_GIT_REPO_PATH}" "${dst_private_git}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_private_git}"
    else
        echo "${dst_private_git} already mounted, skipping bind mount."
    fi

    # The rest of the filesystems are mounted by the callee seeding script.
}

main() {
    local chroot_dir="${1}"
    shift

    local seeding_script="${1}"
    shift

    echo "Starting chroot()." >&2
    echo "chroot_dir=${chroot_dir}, seeding_script=${seeding_script}, args=${*}" >&2
    # Setup the chroot environment (mounts, env vars, etc).
    setup_chroot_env "${chroot_dir}"
    # Execute the seeding script inside the chroot w/exec so it becomes PID 1.
    exec chroot "${chroot_dir}" "${seeding_script}" "${@}"
}

main "${@}"
