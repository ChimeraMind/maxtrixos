#!/bin/bash
# preppers_lib.sh - shared library between all the preppers, to contains functions
#                   that are used OUTSIDE of chroots.
set -eu

source "${MATRIXOS_DEV_DIR:-/matrixos}"/headers/env.include.sh

# START: Vectorized functions. These functions are now living in vector.
# We may want to add small helper commands to vector to execute the checks
# and not have to duplicate their logic here. Preppers library is still executed
# outside of chroots, so we do not have a bootstrapping problem to use them here.

preppers_lib.check_dir_is_root() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "preppers_lib.check_dir_is_root missing parameter." >&2
        return 1
    fi

    # Safety check: Is the inode of the chroot the same as the host root?
    if [[ $(stat -c %i "${chroot_dir}") -eq $(stat -c %i /) ]]; then
        echo "CRITICAL ERROR: CHROOT IS MAPPED TO HOST ROOT. ABORTING." >&2
        exit 1
    fi
}

preppers_lib.check_active_mounts() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "preppers_lib.check_active_mounts missing parameter." >&2
        return 1
    fi

    local active_mounts=
    active_mounts=$(findmnt -rn -o TARGET --submounts --target "${chroot_dir}" \
        | grep "^${chroot_dir}" || true)
    if [ -n "${active_mounts}" ]; then
        echo "[${_seeder_name}] Cannot operate sync to ${chroot_dir}. Active mounts detected:" >&2
        echo "${active_mounts}" >&2
        echo "Please umount them manually." >&2
        return 1
    fi
}

preppers_lib.check_dirs_same_filesystem() {
    local src="${1}"
    local dst="${2}"
    if [ -z "${src}" ] || [ -z "${dst}" ]; then
        echo "preppers_lib.check_dirs_same_filesystem missing parameter." >&2
        return 1
    fi

    local dev1=
    local dev2=
    dev1=$(stat -c '%d' "${src}")
    dev2=$(stat -c '%d' "${dst}")
    [[ "${dev1}" == "${dev2}" ]]
}

preppers_lib.check_fs_capability_support() {
    local test_dir="${1}"
    if [ -z "${test_dir}" ]; then
        echo "preppers_lib.check_fs_capability_support missing parameter." >&2
        return 1
    fi

    local tmp_bin="${test_dir}/.cap_test.$$.bin"
    local tmp_copy="${test_dir}/.cap_test.$$.copy"
    local ret=0

    # Ensure we start clean.
    touch "${tmp_bin}"

    # Try to set the capability.
    if ! setcap 'cap_net_raw+ep' "${tmp_bin}" 2>/dev/null; then
        echo "WARNING: System/FS does not allow setting capabilities." >&2
        rm -f "${tmp_bin}"
        return 1
    fi

    # Copy with archive flags.
    cp -a "${tmp_bin}" "${tmp_copy}" 2>/dev/null

    # Flexible check for the capability string.
    if ! getcap "${tmp_copy}" | grep -q "cap_net_raw[=+]ep"; then
        ret=1
    fi

    rm -f "${tmp_bin}" "${tmp_copy}"
    return "${ret}"
}

preppers_lib.cp_reflink_copy_allowed() {
    local src="${1}"
    local dst="${2}"
    local use_cp_flag="${3}"
    if [ -z "${src}" ] || [ -z "${dst}" ] || [ -z "${use_cp_flag}" ]; then
        echo "preppers_lib.cp_reflink_copy_allowed missing parameters." >&2
        return 1
    fi

    if [ -z "${use_cp_flag}" ] || [ "${src}" = "/" ]; then
        return 1
    fi

    preppers_lib.check_dirs_same_filesystem "${src}" "${dst}"
    preppers_lib.check_fs_capability_support "${src}"
    preppers_lib.check_fs_capability_support "${dst}"
}

# Verify that hardlinks are preserved between source and destination.
# Returns 0 if hardlinks are intact, 1 if they were duplicated/broken.
preppers_lib.check_hardlink_preservation() {
    local src="${1}"
    local dst="${2}"
    if [ -z "${src}" ] || [ -z "${dst}" ]; then
        echo "preppers_lib.check_hardlink_preservation missing parameter." >&2
        return 1
    fi

    echo "Checking hardlink preservation from ${src} to ${dst}..."

    # 1. Find files with multiple links.
    # 2. Print Inode and Path.
    # 3. Sort numerically by Inode so identical inodes are adjacent.
    # 4. Use awk to find the first pair of lines where the Inode ($1) matches the previous line.
    local test_pair
    test_pair=$(find "${src}" -type f -links +1 -printf '%i %p\n' | sort -k1,1n | awk '
        $1 == last_inode {
            print last_line
            print $0
            exit
        }
        { last_inode = $1; last_line = $0 }
    ')

    if [[ -z "${test_pair}" ]]; then
        echo "WARNING: no hardlinked files found in source. Cannot verify." >&2
        return 0
    fi

    # Extract the paths from the pair
    local file1_src=
    local file2_src=
    file1_src=$(echo "${test_pair}" | sed -n '1p' | cut -d' ' -f2-)
    file2_src=$(echo "${test_pair}" | sed -n '2p' | cut -d' ' -f2-)

    echo "  Verifying pair:"
    echo "    Src 1: ${file1_src}"
    echo "    Src 2: ${file2_src}"

    # Map those paths to the destination
    local rel_path1="${file1_src#$src}"
    local rel_path2="${file2_src#$src}"

    local file1_dst="${dst%/}/${rel_path1#/}"
    local file2_dst="${dst%/}/${rel_path2#/}"

    # Compare Inode numbers in the destination
    local inode1_dst=
    local inode2_dst=
    inode1_dst=$(stat -c '%i' "${file1_dst}" 2>/dev/null)
    inode2_dst=$(stat -c '%i' "${file2_dst}" 2>/dev/null)

    if [[ -z "${inode1_dst}" ]] || [[ -z "${inode2_dst}" ]]; then
        echo "ERROR: unable to determine inode information." >&2
        return 1
    fi

    if [[ "${inode1_dst}" == "${inode2_dst}" ]]; then
        echo "SUCCESS: hardlinks preserved (Inode: ${inode1_dst})."
        return 0
    else
        echo "CRITICAL: hardlinks BROKEN! Files were duplicated." >&2
        echo "  File 1. inode: ${inode1_dst}, file: ${file1_dst}" >&2
        echo "  File 2. inode: ${inode2_dst}, file: ${file2_dst}" >&2
        return 1
    fi
}

preppers_lib.create_temp_file() {
    local parent_dir="${1}"
    local prefix="${2:-tmp}"

    mkdir -p "${parent_dir}"
    local new_path=
    new_path=$(mktemp -p "${parent_dir}" "${prefix}.XXXXXXXXXX")

    if [[ $? -ne 0 || -z "${new_path}" ]]; then
        echo "${0}: failed to create temporary file" >&2
        return 1
    fi
    echo "${new_path}"
}

preppers_lib.seeder_chroot_dir_to_name() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "Missing parameter to seeder_chroot_dir_to_name" >&2
        return 1
    fi
    local seeder_name
    seeder_name=$(basename "${chroot_dir}")
    echo "${seeder_name}"
}

preppers_lib.seeder_lock_dir() {
    local lock_dir="${SEEDER_LOCK_DIR}"
    mkdir -p "${lock_dir}"
    echo "${lock_dir}"
}

preppers_lib.seeder_lock_path() {
    local seeder_name="${1}"
    local lock_dir
    lock_dir="$(preppers_lib.seeder_lock_dir)"
    local lock_file="${lock_dir}/${seeder_name}.lock"
    echo "${lock_file}"
}

preppers_lib.execute_with_seeder_lock() {
    local func="${1}"
    local seeder_name="${2}"
    shift 2

    local lock_path
    lock_path=$(preppers_lib.seeder_lock_path "${seeder_name}")
    echo "Acquiring seeder ${seeder_name} lock via ${lock_path} ... (remove lock and re-run to force)"

    local lock_fd=
    # Do not use a subshell otherwise the global cleanup variables used in trap will not
    # be filled properly. Like: ${MOUNTS} in seeder.
    exec {lock_fd}>"${lock_path}"

    if ! flock -x --timeout "${SEEDER_LOCK_WAIT_SECS}" "${lock_fd}"; then
        echo "Timed out waiting for lock ${lock_path}" >&2
        exec {lock_fd}>&-
        return 1
    fi

    echo "Lock for seeder ${seeder_name}, ${lock_path} on FD ${lock_fd} acquired!"

    # We do NOT use a trap. We rely on standard flow control.
    # If "${func}" crashes (set -e), the script dies and OS closes the FD.
    # If "${func}" returns (success or fail), we capture it.
    "${func}" "${@}"
    local ret=${?}

    # Release the lock.
    exec {lock_fd}>&-
    return ${ret}
}

# END: Vectorized functions.

preppers_lib.get_gpg_keychain_dir() {
    local kc_dir="${SEEDER_GPG_KEYS_DIR}"
    [[ ! -d "${kc_dir}" ]] && mkdir -p "${kc_dir}"
    echo "${kc_dir}"
}

preppers_lib.gpg_verify_file() {
    local filepath="${1}"
    local homedir
    homedir=$(preppers_lib.get_gpg_keychain_dir)
    echo "Verifying ${filepath} ..."
    gpg --homedir="${homedir}" --batch --yes --verify "${filepath}.asc" "${filepath}"
}

preppers_lib.gpg_verify_embedded_signature_file() {
    local filepath="${1}"
    local homedir
    homedir=$(preppers_lib.get_gpg_keychain_dir)
    echo "Verifying ${filepath} ..."
    gpg --homedir="${homedir}" --batch --yes --verify "${filepath}"
}

preppers_lib.gpg_decrypt_file() {
    local filepath="${1}"
    local outfilepath="${2}"
    local homedir
    homedir=$(preppers_lib.get_gpg_keychain_dir)
    echo "Decrypting ${filename} to ${outfilepath} ..."
    gpg --homedir="${homedir}" --batch --yes --decrypt --output "${outfilepath}" "${filepath}"
}

# Uses DOWNLOAD_DIR
preppers_lib.get_download_dir() {
    ddir="${DOWNLOAD_DIR}"
    [[ ! -d "${ddir}" ]] && mkdir -p "${ddir}"
    echo "${ddir}"
}

_is_rootfs_functional() {
    local chroot_dir="${1}"
    test -d "${chroot_dir}"
    test -x "${chroot_dir}/bin/sh"
    test -x "${chroot_dir}/bin/ls"
}

_move_chroot_dir_away() {
    local chroot_dir="${1}"
    echo
    local tmp_chroot_name
    tmp_chroot_name=$(basename "${chroot_dir}")
    local tmp_chroot_dir
    tmp_chroot_dir=$(mktemp -d --suffix="${tmp_chroot_name}" --tmpdir="$(dirname "${chroot_dir}")")
    echo "[${_seeder_name}] Executing mv ${chroot_dir} ${tmp_chroot_dir} ... in another 5 seconds ..."
    sleep 5
    mv "${chroot_dir}" "${tmp_chroot_dir}"
    mkdir -p "${chroot_dir}"
}

preppers_lib.sanity_check_chroot_dir() {
    local chroot_dir="${1}"
    local chroot_resume="${2}"
    local _seeder_name="${3}"

    if [ -z "${chroot_dir}" ]; then
        echo "Missing parameter to server prepper" >&2
        return 1
    fi
    if [ -e "${chroot_dir}" ] && [ ! -d "${chroot_dir}" ]; then
        echo "${chroot_dir} is not a directory ..." >&2
        return 1
    fi
    if [ ! -d "${chroot_dir}" ]; then
        echo "Creating chroot dir: ${chroot_dir} ..."
        # it is harmless if we are in the latter scenarios.
        mkdir -p "${chroot_dir}"
    fi

    preppers_lib.check_dir_is_root "${chroot_dir}"
    preppers_lib.check_active_mounts "${chroot_dir}"

    if [ -n "${chroot_resume}" ] && ! _is_rootfs_functional "${chroot_dir}"; then
        echo "[${_seeder_name}] Root filesystem at ${chroot_dir} is NOT functional ..." >&2
        echo "[${_seeder_name}] But you asked me to resume. You! So, what I am going to do is ..." >&2
        echo "[${_seeder_name}] Moving everything to a temp dir and starting over, in 10 seconds ..." >&2
        local c=
        for c in {1..10}; do
            echo -en "${c}."
            sleep 1
        done
        _move_chroot_dir_away "${chroot_dir}"

    elif [ -n "${chroot_resume}" ] && [ -d "${chroot_dir}" ]; then
        echo "[${_seeder_name}] Skipping stage3 unpacking ..."
        echo "[${_seeder_name}] Attempting to resume seeder in chroot: ${chroot_dir} ..."
        return 0
    elif [ -n "${chroot_resume}" ] && [ ! -d "${chroot_dir}" ]; then
        echo "[${_seeder_name}] Requested a chroot resume but chroot dir: ${chroot_dir} does not exist." >&2
        return 1
    elif [ -z "${chroot_resume}" ] && [ -d "${chroot_dir}" ] && [ -x "${chroot_dir}/bin/ls" ]; then
        echo "[${_seeder_name}] ${chroot_dir} exists and seems to be populated, while resume mode is not set." >&2
        echo "[${_seeder_name}] This seems a very suspicious situation, but I will continue nonetheless in 10 seconds." >&2
        echo "[${_seeder_name}] Press CTRL+C NOW if you think you made a mistake." >&2
        echo "[${_seeder_name}] Moving everything to a temp dir and starting over, in 10 seconds..."
        local c=
        for c in {1..10}; do
            echo -en "${c}."
            sleep 1
        done
        _move_chroot_dir_away "${chroot_dir}"
    fi

}

_get_prep_phase_path() {
    local chroot_dir="${1}"
    local prep_phase="${2}"
    echo "${chroot_dir%/}${PREPPERS_PHASES_STATE_DIR}/${prep_phase}.done"
}

preppers_lib.touch_done_prep_phase() {
    local chroot_dir="${1}"
    local prep_phase="${2}"
    if [ -z "${chroot_dir}" ]; then
        echo "preppers_lib.touch_done_prep_phase missing parameter." >&2
        return 1
    fi
    if [ -z "${prep_phase}" ]; then
        echo "preppers_lib.touch_done_prep_phase missing parameter." >&2
        return 1
    fi


    local prep_path
    prep_path="$(_get_prep_phase_path "${chroot_dir}" "${prep_phase}")"
    mkdir -p "$(dirname "${prep_path}")"
    touch "${prep_path}"
}

preppers_lib.is_prep_phase_done() {
    local chroot_dir="${1}"
    local prep_phase="${2}"
    if [ -z "${chroot_dir}" ]; then
        echo "preppers_lib.is_prep_phase_done missing parameter." >&2
        return 1
    fi
    if [ -z "${prep_phase}" ]; then
        echo "preppers_lib.is_prep_phase_done missing parameter." >&2
        return 1
    fi

    local prep_path
    prep_path="$(_get_prep_phase_path "${chroot_dir}" "${prep_phase}")"
    echo "Checking if prep is already done: ${prep_path}"
    test -f "${prep_path}"
}

preppers_lib.sanity_check_latest_bedrock() {
    local latest_bedrock="${1}"
    if [ -z "${latest_bedrock}" ]; then
        echo "Unable to find latest bedrock chroot dir." >&2
        return 1
    fi

    if [ ! -d "${latest_bedrock}" ]; then
        echo "Latest bedrock ${latest_bedrock} does not exist." >&2
        return 1
    fi
    # More sanity checks.
    local d=
    for d in /dev /proc sys; do
        if [ ! -d "${latest_bedrock}/${d}" ]; then
            echo "Latest bedrock ${latest_bedrock}/${d} does not exist." >&2
            return 1
        fi
    done
}

preppers_lib.create_build_metadata_file() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "preppers_lib.create_build_metadata_file missing chroot_dir parameter." >&2
        return 1
    fi

    local bedrock_chroot_dir="${2}"
    if [ -z "${bedrock_chroot_dir}" ]; then
        echo "preppers_lib.create_build_metadata_file missing bedrock_chroot_dir parameter." >&2
        return 1
    fi

    local bedrock_name
    bedrock_name=$(basename "${bedrock_chroot_dir}")

    local build_metadata="${chroot_dir}/${SEEDER_BUILD_METADATA_FILE}"
    local build_metadata_dir
    build_metadata_dir=$(dirname "${build_metadata}")

    mkdir -p "${build_metadata_dir}"
    echo "Writing build metadata to ${build_metadata} ..."
    echo "BEDROCK_ORIGIN=${bedrock_name}" > "${build_metadata}"
    # SEEDER_CHROOT_NAME should be available, we just run in a subshell.
    if [ -n "${SEEDER_CHROOT_NAME}" ]; then
        echo "SEED_NAME=${SEEDER_CHROOT_NAME}" >> "${build_metadata}"
    else
        echo "WARNING: SEEDER_CHROOT_NAME not set!" >&2
    fi

    # Persist the stage3 file info if available.
    echo "STAGE3_URL=${STAGE3_URL}" >> "${build_metadata}"
    echo "STAGE3_FILE=${STAGE3_FILE}" >> "${build_metadata}"

    cat "${build_metadata}"
}

preppers_lib._rsync_copy() {
    local src="${1}"
    local dst="${2}"
    echo "Spawning rsync from ${src} to ${dst} ..."
    rsync \
        --archive \
        --verbose \
        --progress \
        --partial \
        -HAX \
        --numeric-ids \
        --delete-during \
        --one-file-system \
        "${src%/}/" "${dst%/}/"
}

preppers_lib._cp_reflink_copy() {
    local src="${1}"
    local dst="${2}"

    echo "Removing ${dst} ..."
    # It is safe to remove dst because parent func already checked if we have
    # active mounts.
    rm -rf "${dst}"

    echo "Spawning cp --preserve=links --reflink=auto from ${src} to ${dst} ..."
    cp -a --preserve=links --reflink=auto "${src}" "${dst}"
}

preppers_lib._rsync_from_bedrock() {
    local chroot_dir="${1}"
    local latest_bedrock="${2}"
    if [ -z "${latest_bedrock}" ]; then
        echo "preppers_lib._rsync_from_bedrock missing parameter." >&2
        return 1
    fi
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir not set for prepper." >&2
        return 1
    fi

    # We need to create chroot_dir if we check for caps.
    mkdir -p "${chroot_dir}"

    # Active mounts check already done by sanity_check_chroot_dir.

    local use_cp="${USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC}"

    if preppers_lib.cp_reflink_copy_allowed "${latest_bedrock}" "${chroot_dir}" "${use_cp}"; then
        echo "Using experimental cp --reflink=auto copy mode ..."
        preppers_lib._cp_reflink_copy "${latest_bedrock}" "${chroot_dir}"
    else
        preppers_lib._rsync_copy "${latest_bedrock}" "${chroot_dir}"
    fi
    preppers_lib.check_hardlink_preservation "${latest_bedrock}" "${chroot_dir}"

    preppers_lib.create_build_metadata_file "${chroot_dir}" "${latest_bedrock}"
}

preppers_lib.rsync_from_bedrock() {
    local chroot_dir="${1}"
    local latest_bedrock="${2}"
    if [ -z "${latest_bedrock}" ]; then
        echo "preppers_lib.rsync_from_bedrock missing parameter." >&2
        return 1
    fi
    if [ -z "${chroot_dir}" ]; then
        echo "chroot_dir not set for prepper." >&2
        return 1
    fi

    # Lock bedrock.
    local seeder_name
    seeder_name=$(preppers_lib.seeder_chroot_dir_to_name "${latest_bedrock}")
    preppers_lib.execute_with_seeder_lock "preppers_lib._rsync_from_bedrock" "${seeder_name}" \
        "${chroot_dir}" "${latest_bedrock}"
}

_prepper_dir="$(dirname "${0}")"
_seeders_dir="$(dirname "${_prepper_dir}")"

preppers_lib.find_latest_bedrock_chroot_dir() {
    (
        # Import the bedrock params in a subshell to avoid poisoning.
        source "${_seeders_dir}"/00-bedrock/params.sh
        bedrock_params.find_latest_chroot_dir "00-bedrock"
    )
}