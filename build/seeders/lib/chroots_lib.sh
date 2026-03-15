#!/bin/bash
# chroots_lib.sh - shared library between all the chroot seeders. This is also meant to share
#                  state between chroot.sh scripts and only executed inside seeding chroots.
set -eu

# Used by maybe_mount_common_filesystems.
MOUNTS=()


_get_phase_path() {
    echo "${SEEDERS_PHASES_STATE_DIR}/${1}.done"
}

chroots_lib.touch_done_phase() {
    local phase_path=
    phase_path="$(_get_phase_path "${1}")"
    mkdir -p "$(dirname "${phase_path}")"
    touch "${phase_path}"
}

chroots_lib.is_phase_done() {
    local phase_path=
    phase_path="$(_get_phase_path "${1}")"
    echo "Checking if phase is already done: ${phase_path}"
    test -f "${phase_path}"
}

chroots_lib.package_list_path() {
    local seeder_name="${1}"
    if [ -z "${seeder_name}" ]; then
        echo "${0}: missing seeder name parameter" >&2
        return 1
    fi
    echo "${MATRIXOS_DEV_DIR}/build/seeders/${seeder_name}/packages.conf"
}

chroots_lib.portage_confdir_path() {
    local seeder_name="${1}"
    if [ -z "${seeder_name}" ]; then
        echo "${0}: missing seeder name parameter" >&2
        return 1
    fi
    echo "${MATRIXOS_DEV_DIR}/build/seeders/${seeder_name}/portage"
}

chroots_lib.validate_package_list_path() {
    local path="${1}"
    if [ -z "${path}" ]; then
        echo "${0}: missing parameter" >&2
        return 1
    fi
    if [ ! -e "${path}" ]; then
        echo "${path} does not exist." >&2
        return 1
    fi
}

chroots_lib.validate_portage_confdir_path() {
    local path="${1}"
    if [ -z "${path}" ]; then
        echo "${0}: missing parameter" >&2
        return 1
    fi
    if [ ! -d "${path}" ]; then
        echo "${path} does not exist." >&2
        return 1
    fi
}

chroots_lib.validate_matrixos_git_repo() {
    # matrixos.git is taken care of by seeder.sh, but check.
    local matrixos_dev_dir_flag="${MATRIXOS_DEV_DIR}/.matrixos"
    if [ ! -f "${matrixos_dev_dir_flag}" ]; then
        echo "${matrixos_dev_dir_flag} does not exist. matrixos.git must be cloned into ${MATRIXOS_DEV_DIR}." >&2
        return 1
    fi
}

_mos_private_message() {
    echo "Please set it in conf/matrixos.conf, matrixOS.PrivateGitRepoPath." >&2
    echo "See README.md and https://github.com/lxnay/matrixos-private-example for more details." >&2
    echo "This directory contains YOUR GPG private keys and SecureBoot certs necessary to build" >&2
    echo "and release a custom matrixOS Gentoo build." >&2
}

chroots_lib.check_matrixos_private() {
    local matrixos_private="${1}"
    if [ -z "${matrixos_private}" ]; then
        echo "matrixOS.PrivateGitRepoPath is empty ..." >&2
        _mos_private_message
        return 1
    fi
    if [ ! -d "${matrixos_private}" ]; then
        echo "${matrixos_private} does not exist ..." >&2
        _mos_private_message
        return 1
    fi
}

chroots_lib.validate_matrixos_private() {
    # Inside chroots, we always place matrixos-private into /etc/matrixos-private.
    # This is because many pieces of the codebase, including the Portage config,
    # expect it to be there.
    local matrixos_private="${DEFAULT_PRIVATE_GIT_REPO_PATH}"
    chroots_lib.check_matrixos_private "${matrixos_private}"

    # This is usually bind-mount. Make sure it is and not
    # copied over.
    local mounted=
    mounted=$(findmnt -n -o TARGET "${matrixos_private}")
    if [ "${mounted}" != "${matrixos_private}" ]; then
        echo "${matrixos_private} is not a bind-mount. seeder should do this." >&2
        return 1
    fi
}

chroots_lib.cleanup() {
    chroots_lib.maybe_umount_common_filesystems
}

chroots_lib._maybe_mount_sys() {
    if ! mountpoint -q /sys; then
        echo "Mounting sysfs on /sys inside chroot since it was not mounted by the host..." >&2
        mkdir -p /sys
        MOUNTS+=( "/sys" )
        mount -t sysfs sys /sys
    else
        echo "/sys is already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

chroots_lib._maybe_mount_dev() {
    if ! mountpoint -q /dev; then
        echo "Mounting devtmpfs on /dev inside chroot since it was not mounted by the host..." >&2
        mkdir -p /dev

        MOUNTS+=( "/dev" )
        mount -t devtmpfs devtmpfs /dev

        echo "Mounting devpts on /dev/pts inside chroot since it was not mounted by the host..." >&2
        MOUNTS+=( "/dev/pts" )
        mount -t devpts devpts /dev/pts -o rw,nosuid,noexec,relatime,gid=5,mode=600,ptmxmode=000

        echo "Mounting tmpfs on /dev/shm inside chroot since it was not mounted by the host..." >&2
        MOUNTS+=( "/dev/shm" )
        mount -t tmpfs devshm /dev/shm -o rw,nosuid,nodev,mode=1777
    else
        echo "/dev is already mounted on the host, skipping mounting it inside chroot." >&2
    fi
}

chroots_lib.maybe_mount_common_filesystems() {
    chroots_lib._maybe_mount_sys
    chroots_lib._maybe_mount_dev
}

chroots_lib.maybe_umount_common_filesystems() {
    local m=
    # Umount in reverse order.
    for (( idx=${#MOUNTS[@]}-1 ; idx>=0 ; idx-- )) ; do
        m="${MOUNTS[idx]}"
        if mountpoint -q "${m}"; then
            echo "Unmounting ${m} inside chroot..." >&2
            umount -l "${m}"
        else
            echo "${m} is not mounted, skipping unmount." >&2
        fi
    done
}

chroots_lib.default_portage_bootstrap() {
    for repo in "${@}"; do
        eselect repository enable "${repo}"
    done
    for repo in "${@}"; do
        emaint --repo="${repo}" sync
    done
}

chroots_lib.default_buildenv_bootstrap() {
    local seeder_name="${1}"
    if [ -z "${seeder_name}" ]; then
        echo "${0}: missing seeder name parameter" >&2
        return 1
    fi
    local package_list_path=
    package_list_path=$(chroots_lib.package_list_path "${seeder_name}")
    chroots_lib.validate_package_list_path "${package_list_path}"

    local portage_confdir_path=
    portage_confdir_path="$(chroots_lib.portage_confdir_path "${seeder_name}")"
    chroots_lib.validate_portage_confdir_path "${portage_confdir_path}"
    chroots_lib.validate_matrixos_git_repo

    # Set up portage config.
    local etcportage="/etc/portage"
    rm -rf "${etcportage}"
    ln -sf "..${portage_confdir_path}" "${etcportage}"
    echo "New ${etcportage} path:"
    ls -la "${etcportage}"
    if ! [[ -L "${etcportage}" && -d "${etcportage}" ]]; then
        echo "${etcportage} is not a valid directory symlink." >&2
        return 1
    fi
    # Setup and validate matrixos-private.
    chroots_lib.validate_matrixos_private
}

chroots_lib.default_build_everything() {
    local seeder_name="${1}"
    if [ -z "${seeder_name}" ]; then
        echo "${0}: missing seeder name parameter" >&2
        return 1
    fi
    local package_list_path=
    package_list_path=$(chroots_lib.package_list_path "${seeder_name}")
    chroots_lib.validate_package_list_path "${package_list_path}"

    local packages=()
    # Read the file, skip comments and spaces.
    readarray -t packages < <(grep -vE '^[[:space:]]*($|#)' "${package_list_path}")
    echo "Building the package list, containing:"
    local p=
    for p in "${packages[@]}"; do
        echo ">> ${p}"
    done
    chroots_lib.generic_build --newuse -v "${packages[@]}"
}

chroots_lib.default_clean_temporary_artifacts() {
    echo "Cleaning temporary artifacts ..."
    dirs=(
        /var/tmp/portage
        /var/cache/revdep-rebuild
        /root/.cache
    )
    for d in "${dirs[@]}"; do
        echo "Removing ${d} inside chroot ..."
        rm -rf "${d}"
    done

    # /var/cache/distfiles, binpkgs should be bind mount. /var/db/repos is copied over.
    env-update
}

chroots_lib.rebuild_before_portage_counter() {
    local counter="${1}"
    if [ -z "${counter}" ]; then
        echo "${0}: missing parameter to rebuild_before_portage_counter" >&2
        return 1
    fi

    local rebuilds=()
    local cntfile=
    local cnt=
    local cntdir=
    local cat=
    local pf=
    local repo=
    local pkg=
    local atom=
    local pn=
    for cntfile in /var/db/pkg/*/*/COUNTER; do
        cnt=$(cat "${cntfile}")
        if [[ ${cnt} -le ${counter} ]]; then
            cntdir=$(dirname "${cntfile}")
            pf=$(basename "${cntdir}")
            cat=$(basename "$(dirname "${cntdir}")")
            repo=$(cat "${cntdir}/repository")
            atom="${cat}/${pf}"
            slot=$(portageq metadata / installed "${atom}" SLOT)
            pn=$(portageq metadata / installed "${atom}" PN)
            pkg="${cat}/${pn}:${slot}::${repo}"
            echo ">> ${pkg}"
            rebuilds+=( "${pkg}" )
        fi
    done
    if [[ ${#rebuilds[@]} -gt 0 ]]; then
        chroots_lib.generic_build -v --oneshot "${rebuilds[@]}"
    else
        echo "No packages to rebuild."
    fi
}

_get_counter_path() {
    local counter_path="${SEEDERS_PHASES_STATE_DIR}/.portage_counter.tmp"
    echo "${counter_path}"
}

chroots_lib.store_portage_counter() {
    local counter="${1}"
    if [ -z "${counter}" ]; then
        echo "${0}: missing parameter to store_portage_counter" >&2
        return 1
    fi

    local counter_path=
    counter_path=$(_get_counter_path)
    mkdir -p "$(dirname "${counter_path}")"
    echo "${counter}" > "${counter_path}"
}

chroots_lib.get_stored_portage_counter() {
    local counter_path=
    counter_path=$(_get_counter_path)
    if [ -f "${counter_path}" ]; then
        cat "${counter_path}"
    else
        echo -en "-1"
    fi
}

chroots_lib.get_current_portage_counter() {
    (for f in /var/db/pkg/*/*/COUNTER; do cat "${f}"; echo; done) | sort -n | tail -n 1
}

chroots_lib.try_get_emerge_jobs_flags() {
    local num_procs=
    num_procs=$(nproc || true)
    # Assume 1C/2G
    num_gib=$(free -g | awk '/^Mem:/{print $2}' || true)

    if [ -z "${num_procs}" ] || [ -z "${num_gib}" ]; then
        echo "WARNING: Could not determine number of processors or amount of memory. Using default 2C/4G." >&2
        num_procs=2
        num_gib=4
    else
        # Normalize num_procs based on memory, to avoid OOMs.
        # For example, on a 4GiB RAM machine, we don't want to
        # spawn 8 emerge processes just because there are 8 cores.
        local num_gib_procs=
        num_gib_procs=$(( num_gib / 2 ))
        # If num_gib_procs is odd, make it even
        if [ $(( num_gib_procs % 2 )) -ne 0 ]; then
            num_gib_procs=$(( num_gib_procs + 1 ))
        fi
        if [ "${num_gib_procs}" -lt "${num_procs}" ]; then
            echo "WARNING: Limiting emerge jobs to ${num_gib_procs} based on available memory (${num_gib} GiB)." >&2
            num_procs="${num_gib_procs}"
        fi
    fi

    local flags=()
    if [ -n "${num_procs}" ]; then
        flags+=(
            --jobs="${num_procs}"
            --load-average="${num_procs}"
        )
    fi
    echo "${flags[@]}"
}

chroots_lib.emerge_common_args() {
    local jobs_flags
    read -ra jobs_flags <<< "$(chroots_lib.try_get_emerge_jobs_flags)"
    local args=(
        --binpkg-respect-use=y
        --buildpkg
        --usepkg
        --quiet-build=y
        --verbose
    )
    echo "${args[@]}" "${jobs_flags[@]}"
}

chroots_lib.emerge_common_rebuild_args() {
    local jobs_flags
    read -ra jobs_flags <<< "$(chroots_lib.try_get_emerge_jobs_flags)"
    local args=(
        --quiet-build=y
        --verbose
    )
    echo "${args[@]}" "${jobs_flags[@]}"
}

chroots_lib.generic_build() {
    env-update
    local common_args
    read -ra common_args <<< "$(chroots_lib.emerge_common_args)"

    echo ">> emerge" "${common_args[@]}" "${@}"
    emerge "${common_args[@]}" "${@}"
}

chroots_lib.generic_forced_rebuild() {
    env-update
    local common_args
    read -ra common_args <<< "$(chroots_lib.emerge_common_rebuild_args)"

    echo ">> emerge (forcing rebuild)" "${common_args[@]}" "${@}"
    emerge "${common_args[@]}" "${@}"
}
