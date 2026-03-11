#!/bin/bash
set -eu

# The dating scheme is determined by SEEDER_DATE_CADENCE (daily, weekly, monthly).
# - daily:   YYYYMMDD of today.
# - weekly:  YYYYMMDD anchored to the most recent Monday (default).
# - monthly: YYYYMM01 anchored to the first day of the current month.
params_lib.get_chroot_date() {
    local cadence="${SEEDER_DATE_CADENCE:-weekly}"
    case "${cadence}" in
        daily)
            date +%Y%m%d
            ;;
        weekly)
            date -d "$(( $(date +%u) - 1 )) days ago" +%Y%m%d
            ;;
        monthly)
            date +%Y%m01
            ;;
        *)
            echo "params_lib.get_chroot_date: invalid SEEDER_DATE_CADENCE '${cadence}'" >&2
            return 1
            ;;
    esac
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