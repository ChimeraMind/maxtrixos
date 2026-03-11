#!/bin/bash
set -eu

# The default dating scheme is "YYYYMMDD" anchored to the first past Monday.
params_lib.get_chroot_date() {
    date -d "$(( $(date +%u) - 1 )) days ago" +%Y%m%d
}

params_lib.get_chroot_seeder_done_flag_file() {
    local seeder_name="${1}"
    if [ -z "${seeder_name}" ]; then
        echo "get_chroot_seeder_done_flag_file: missing seeder name parameter" >&2
        return 1
    fi
    local chroot_dir="${2}"
    if [ -z "${chroot_dir}" ]; then
        echo "get_chroot_seeder_done_flag_file: missing chroot dir parameter" >&2
        return 1
    fi

    local prefix="${SEEDER_DONE_FLAG_FILE_PREFIX}"
    if [ -z "${prefix}" ]; then
        echo "get_chroot_seeder_done_flag_file: missing SEEDER_DONE_FLAG_FILE_PREFIX env var" >&2
        return 1
    fi
    local state_dir="${SEEDERS_PHASES_STATE_DIR}"
    if [ -z "${state_dir}" ]; then
        echo "get_chroot_seeder_done_flag_file: missing SEEDERS_PHASES_STATE_DIR env var" >&2
        return 1
    fi
    local seeder_done_flag_file="${state_dir}/${prefix}"

    local flag_path="${chroot_dir%/}${seeder_done_flag_file}_${seeder_name}"
    echo "${flag_path}"
}