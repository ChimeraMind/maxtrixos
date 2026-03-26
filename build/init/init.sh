#!/bin/bash
# This script is used to prepare the chroot environment for a seed, release or image
# and then execute the chroot() syscall to start the seeding process.
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
# Runner: seeder.
#
# SEEDER_DISTFILES_DIR
# The path to the distfiles directory from outside the chroot. This is expected
# to be bind mount inside the chroot at /var/cache/distfiles if not already
# mounted.
# Runner: seeder.
#
# SEEDER_BINPKGS_DIR
# The path to the binpkgs directory from outside the chroot. This is expected
# to be bind mount inside the chroot at /var/cache/binpkgs if not already
# mounted.
# Runner: seeder.
#
# IMAGER_EFI_MOUNT
# The path to the EFI partition mount point from outside the chroot. This is expected
# to be bind mount inside the chroot at /efi if not already mounted.
# Runner: imager.
#
# IMAGER_BOOT_MOUNT
# The path to the boot partition mount point from outside the chroot. This is expected
# to be bind mount inside the chroot at /boot if not already mounted.
# Runner: imager.
#
# IMAGER_EFI_ROOT
# The path to the EFI partition root from the perspective of the chroot.
# Runner: imager.
#
# IMAGER_BOOT_ROOT
# The path to the boot partition root from the perspective of the chroot.
# Runner: imager.
#
# RUNNER_TYPE
# The type of the runner executing the seeding process. This is used to
# determine who is calling us and act accordingly. For example, if the runner is the Releaser,
# we know that we are in a release context.
# Valid values are: "seeder", "releaser", "imager".
#
# This script takes the following arguments:
# $1: The path to the chroot dir.
# $2: The path to the seeding script to execute inside the chroot.
# $3..$n: The arguments to pass to the seeding script.
set -eu

maybe_mount_distfiles() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount distfiles." >&2
        return 1
    fi

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
}

maybe_mount_binpkgs() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount binpkgs." >&2
        return 1
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
}

maybe_mount_private_git() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount private git repository." >&2
        return 1
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
}

maybe_mount_sys() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount sysfs." >&2
        return 1
    fi

    local dst_sys="${chroot_dir%/}/sys"
    mkdir -p "${dst_sys}"
    if ! mountpoint -q "${dst_sys}"; then
        echo "Mounting sysfs on ${dst_sys} ..."
        mount -t sysfs sys "${dst_sys}"
    else
        echo "${dst_sys} is already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

maybe_mount_dev() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount dev." >&2
        return 1
    fi

    local dst_dev="${chroot_dir%/}/dev"
    if ! mountpoint -q "${dst_dev}"; then
        echo "Mounting devtmpfs on ${dst_dev} ..."

        mkdir -p "${dst_dev}"
        mount -t devtmpfs devtmpfs "${dst_dev}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_dev}"

        echo "Mounting devpts on ${dst_dev}/pts ..."
        mount -t devpts devpts "${dst_dev}/pts" -o rw,nosuid,noexec,relatime,gid=5,mode=600,ptmxmode=000
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_dev}/pts"

        echo "Mounting tmpfs on ${dst_dev}/shm ..."
        mount -t tmpfs devshm "${dst_dev}/shm" -o rw,nosuid,nodev,mode=1777
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_dev}/shm"
    else
        echo "${dst_dev} is already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

maybe_mount_run_lock() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount /run/lock." >&2
        return 1
    fi

    local dst_run_lock="${chroot_dir%/}/run/lock"
    if ! mountpoint -q "${dst_run_lock}"; then
        echo "Mounting tmpfs on ${dst_run_lock} ..."

        mkdir -p "${dst_run_lock}"
        mount -t tmpfs run "${dst_run_lock}" -o rw,nosuid,nodev,noexec,relatime,size=5M
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_run_lock}"
    else
        echo "${dst_run_lock} is already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

maybe_mount_efi() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount EFI partition." >&2
        return 1
    fi

    if [ -z "${IMAGER_EFI_MOUNT:-}" ]; then
        echo "IMAGER_EFI_MOUNT is not set, unable to setup chroot environment for imager." >&2
        return 1
    fi

    local dst_efi="${chroot_dir%/}${IMAGER_EFI_ROOT}"
    mkdir -p "${dst_efi}"
    if ! mountpoint -q "${dst_efi}"; then
        echo "Mounting ${IMAGER_EFI_MOUNT} on ${dst_efi} ..."
        mount --bind "${IMAGER_EFI_MOUNT}" "${dst_efi}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_efi}"
    else
        echo "${dst_efi} already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

maybe_mount_boot() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount boot partition." >&2
        return 1
    fi

    if [ -z "${IMAGER_BOOT_MOUNT:-}" ]; then
        echo "IMAGER_BOOT_MOUNT is not set, unable to setup chroot environment for imager." >&2
        return 1
    fi

    local dst_boot="${chroot_dir%/}${IMAGER_BOOT_ROOT}"
    mkdir -p "${dst_boot}"
    if ! mountpoint -q "${dst_boot}"; then
        echo "Mounting ${IMAGER_BOOT_MOUNT} on ${dst_boot} ..."
        mount --bind "${IMAGER_BOOT_MOUNT}" "${dst_boot}"
        # This is not necessary starting util-linux 2.27 since unshare
        # enforces --propagation private by default. But let's keep this for
        # a bit to be on the safe side.
        mount --make-private "${dst_boot}"
    else
        echo "${dst_boot} already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

maybe_mount_cgroup2() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to mount cgroup2." >&2
        return 1
    fi

    local dst_cgroup="${chroot_dir%/}/sys/fs/cgroup"
    mkdir -p "${dst_cgroup}"
    if ! mountpoint -q "${dst_cgroup}"; then
        echo "Mounting cgroup2 on ${dst_cgroup} ..."
        mount -t cgroup2 none "${dst_cgroup}"
    else
        echo "${dst_cgroup} already mounted, skipping cgroup2 mount." >&2
    fi
}

setup_chroot_env() {
    local chroot_dir="${1}"

    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to setup chroot environment." >&2
        return 1
    fi

    local runner_type="${RUNNER_TYPE:-}"
    if [ -z "${runner_type}" ]; then
        echo "RUNNER_TYPE is not set, unable to setup chroot environment." >&2
        return 1
    fi
    echo "init.sh: Runner type is ${runner_type}."

    if [ "${runner_type}" = "seeder" ]; then
        maybe_mount_distfiles "${chroot_dir}"
        maybe_mount_binpkgs "${chroot_dir}"
        maybe_mount_private_git "${chroot_dir}"
    fi
    if [ "${runner_type}" = "imager" ]; then
        maybe_mount_efi "${chroot_dir}"
        maybe_mount_boot "${chroot_dir}"
    fi

    maybe_mount_sys "${chroot_dir}"
    maybe_mount_cgroup2 "${chroot_dir}"
    maybe_mount_dev "${chroot_dir}"
    maybe_mount_run_lock "${chroot_dir}"
    # The rest of the filesystems are mounted by the callee target executable.
}

setup_cgroups() {
    chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir is not set, unable to setup cgroups." >&2
        return 1
    fi

    # Print worker resource diagnostics.
    # We read from the chroot-relative cgroup2 mount
    # (${chroot_dir}/sys/fs/cgroup/) which was mounted by
    # maybe_mount_cgroup2() and reflects the worker's cgroup namespace.
    local cg_root="${chroot_dir%/}/sys/fs/cgroup"
    local cg_mem_max="${cg_root}/memory.max"
    local cg_cpu_max="${cg_root}/cpu.max"
    local cg_cpuset="${cg_root}/cpuset.cpus.effective"

    if [ -f "${cg_mem_max}" ]; then
        local max_bytes
        max_bytes=$(cat "${cg_mem_max}" 2>/dev/null || true)
        if [ -n "${max_bytes}" ] && [ "${max_bytes}" != "max" ]; then
            local mem_gib=$(( max_bytes / 1073741824 ))
            echo "init.sh: cgroup memory.max=${max_bytes} (${mem_gib} GiB)" >&2
        else
            echo "init.sh: cgroup memory.max=${max_bytes:-N/A}" >&2
        fi
    else
        echo "init.sh: cgroup memory.max=N/A (${cg_mem_max} not enabled)" >&2
    fi

    if [ -f "${cg_cpu_max}" ]; then
        echo "init.sh: cgroup cpu.max=$(cat "${cg_cpu_max}" 2>/dev/null)" >&2
    else
        echo "init.sh: cgroup cpu.max=N/A (${cg_cpu_max} not enabled)" >&2
    fi
    if [ -f "${cg_cpuset}" ]; then
        echo "init.sh: cgroup cpuset.cpus=$(cat "${cg_cpuset}" 2>/dev/null)" >&2
    else
        echo "init.sh: cgroup cpuset.cpus=N/A (${cg_cpuset} not enabled)" >&2
    fi
}

main() {
    local chroot_dir="${1}"
    shift

    local target_exec="${1}"
    shift

    echo "Starting chroot()." >&2
    echo "chroot_dir=${chroot_dir}, target_exec=${target_exec}, args=${*}" >&2
    # Setup the chroot environment (mounts, env vars, etc).
    setup_chroot_env "${chroot_dir}"
    setup_cgroups "${chroot_dir}"

    # Execute the target executable inside the chroot w/exec so it becomes PID 1.
    exec chroot "${chroot_dir}" "${target_exec}" "${@}"
}

main "${@}"
