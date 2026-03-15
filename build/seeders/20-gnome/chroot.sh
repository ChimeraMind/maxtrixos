#!/bin/bash
# This is the main script that is executed inside the chroot for a seeder.
# It is responsible for executing the different phases of the seeding process, and
# for keeping track of which phases have been completed, so that if the process
# is interrupted, it can be resumed from the last completed phase.
# At the end of the process, the chroot should be in a state where it can be used as
# a base for derived seeders, or for generating artifacts (like a bootable image, a
# Content Delivery System release, etc).
#
# Variables exported by the worker (vector/lib/seeder/worker.go) and available in
# the environment:
#
# MATRIXOS_DEV_DIR
# The path to the matrixOS dev directory from within the chroot, which is the root of the
# matrixOS repository, and the main directory where seeders should read/write data.
#
# SEEDERS_PHASES_STATE_DIR: the path to the directory where seeders can read/write files to
# keep track of which phases have been completed. This is used by the chroot.sh scripts
# to implement idempotency and resumability.
#
set -eu

source "${MATRIXOS_DEV_DIR}/build/seeders/headers/seedersenv.include.sh"

source "${MATRIXOS_DEV_DIR}/build/seeders/lib/chroots_lib.sh"

# TODO: maybe we can infer the kernel from the package list.
BUILD_KERNEL_PACKAGES=(
    sys-kernel/matrixos-kernel::matrixos
)
UPSTREAM_PORTAGE_REPOS=(
    steam-overlay
    guru
)

_seeder_name=$(basename "$(dirname "${0}")")


gnome.buildenv_bootstrap() {
    chroots_lib.default_buildenv_bootstrap "${_seeder_name}"
}

gnome.portage_bootstrap() {
    chroots_lib.default_portage_bootstrap "${UPSTREAM_PORTAGE_REPOS[@]}"
}

gnome.build_everything() {
    chroots_lib.default_build_everything "${_seeder_name}"
    # Trigger a rebuild of the kernel so that we bundle the latest and
    # correct initramfs setup.
    chroots_lib.generic_forced_rebuild "${BUILD_KERNEL_PACKAGES[@]}"
}

gnome.clean_temporary_artifacts() {
    chroots_lib.default_clean_temporary_artifacts

    # Clean stale distfiles
    eclean-dist
    eclean-pkg
}

gnome.tweak_nsswitch() {
    # make the default /etc/nsswitch.conf a bit less dumb
    # and add support for dns and mdns resolution.
    # This is done here because it's tied to the portage setup.
    sed -i '/^hosts:/c\hosts:      files myhostname mymachines dns mdns_minimal [NOTFOUND=return] resolve [!UNAVAIL=return]' \
        "/etc/nsswitch.conf"
}

gnome.tweak_resolved() {
    # Disable multicast DNS support in systemd-resolved as atm
    # avahi-daemon is providing it.
    local resolved_conf="/etc/systemd/resolved.conf"
    if [ -f "${resolved_conf}" ]; then
        echo "# matrixOS uses avahi for Multicast DNS." >> "${resolved_conf}"
        echo "MulticastDNS=no" >> "${resolved_conf}"
    fi
}

main() {

    chroots_lib.maybe_mount_common_filesystems

    local phases=(
        gnome.buildenv_bootstrap
        gnome.portage_bootstrap
        gnome.build_everything
        gnome.tweak_nsswitch
        gnome.tweak_resolved
        gnome.clean_temporary_artifacts
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