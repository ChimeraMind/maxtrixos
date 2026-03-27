#!/bin/bash
# This is the main script that is executed inside the chroot for a seeder
# after the seeder has been built. It is always executed, like chroot.sh.
#
# Variables exported by the worker (vector/lib/seeder/worker.go) and available in
# the environment:
#
# MATRIXOS_DEV_DIR
# The path to the matrixOS dev directory from within the chroot, which is the root of the
# matrixOS repository, and the main directory where seeders should read/write data.
#
# SEEDER_OVERLAY_GIT_REPO
# The Git repository URL for the seeder overlay, which is a repository containing the ebuilds
# and configs for the seeder. This is used by the chroot scripts to setup the
# Portage overlay for the seeder.
#
# DEFAULT_PRIVATE_GIT_REPO_PATH
# The directory path to the private git repository. This directory is expected to
# be already empty at this stage.
#
# SEEDERS_PHASES_STATE_DIR: the path to the directory where seeders can read/write files to
# keep track of which phases have been completed. This is used by the chroot.sh scripts
# to implement idempotency and resumability.
#
# SEEDER_DONE_FLAG_FILE_PREFIX
# The prefix for the seeder done flag file, which is used by the chroot.sh scripts
# to determine if a seeder has completed its process and the chroot is in a state
# that can be used as a base for derived seeders.
#
# SEEDER_DATE_CADENCE
# The cadence at which seeder chroots are versioned. This is used to determine the
# name of the chroot directory for the seeder, and to determine when to create a new
# chroot or reuse an existing one.
#
set -e
if [ -e /etc/profile ]; then
    source /etc/profile
fi
set -eu

source "${MATRIXOS_DEV_DIR}/build/seeders/lib/chroots_lib.sh"


gnome_poster.clean_artifacts() {
    chroots_lib.default_clean_temporary_artifacts

    # Clean stale distfiles
    eclean-dist
    eclean-pkg
}

main() {
    cd /

    chroots_lib.setup

    local phases=(
        gnome_poster.clean_artifacts
    )

    # Pre-run tests to check that for every phase we have a function declared
    for phase in "${phases[@]}"; do
        if ! declare -F "${phase}"; then
            echo "Function ${phase} does not exist." >&2
            return 1
        fi
    done

    for phase_f in "${phases[@]}"; do
        if ! chroots_lib.is_phase_done "${phase_f}"; then
            echo "Executing phase: ${phase_f} ..."
            "${phase_f}"
            chroots_lib.touch_done_phase "${phase_f}"
        else
            echo "${phase_f} already finished."
        fi
    done
}

main "${@}"
