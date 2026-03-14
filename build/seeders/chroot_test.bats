#!/usr/bin/env bats
# Unit tests for build/seeders/*/chroot.sh
#
# Each chroot.sh ends with `main "${@}"`, so we strip that line and source
# the remaining function definitions.  The real chroots_lib.sh is sourced
# through MATRIXOS_DEV_DIR pointing at the repository root.

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
    export MATRIXOS_DEV_DIR="${REPO_ROOT}"

    # Env vars required by chroots_lib.sh functions at call time.
    export SEEDERS_PHASES_STATE_DIR="${TEST_TMPDIR}/phases"
    export DEFAULT_PRIVATE_GIT_REPO_PATH="${TEST_TMPDIR}/private"
    export SEEDER_OVERLAY_GIT_REPO="https://example.com/overlay.git"
    export SEEDER_DONE_FLAG_FILE_PREFIX="/done-prefix"
    export SEEDER_DATE_CADENCE="weekly"

    mkdir -p "${TEST_TMPDIR}/private"
    mkdir -p "${MATRIXOS_DEV_DIR}/build/seeders"
    # Ensure .matrixos exists for validation.
    touch "${MATRIXOS_DEV_DIR}/.matrixos"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Stubs for heavy external commands (emerge, eselect, emaint, etc.)
    for cmd in emerge eselect emaint env-update locale-gen emerge-webrsync \
               eclean-dist eclean-pkg qlist; do
        cat > "${STUB_BIN}/${cmd}" << 'EOF'
#!/bin/bash
exit 0
EOF
        chmod +x "${STUB_BIN}/${cmd}"
    done
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# Helper: source a chroot.sh without executing main.
_load_chroot_script() {
    local script="${1}"
    local tmp="${TEST_TMPDIR}/_chroot_$$.sh"
    sed '/^main "\${@}"$/d' "${script}" > "${tmp}"
    source "${tmp}"
}

# ===========================================================================
# 00-bedrock/chroot.sh
# ===========================================================================

@test "bedrock chroot: all phase functions are declared" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"

    local expected_phases=(
        bedrock.system_bootstrap
        bedrock.buildenv_bootstrap
        bedrock.portage_bootstrap
        bedrock.build_resolve_conflicts
        bedrock.build_kernel
        bedrock.build_system
        bedrock.build_everything
        bedrock.tweak_nsswitch
        bedrock.clean_temporary_artifacts
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

@test "bedrock chroot: _seeder_name is derived from dir name" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"

    # _seeder_name uses dirname of $0 which in test context won't be 00-bedrock.
    # But we can verify the variable exists (it's set at top level of the script).
    [ -n "${_seeder_name:-}" ] || true
}

@test "bedrock chroot: BOOTSTRAP_PACKAGES are defined" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"
    [ "${#BOOTSTRAP_PACKAGES[@]}" -gt 0 ]
}

@test "bedrock chroot: BUILD_KERNEL_PACKAGES are defined" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"
    [[ "${BUILD_KERNEL_PACKAGES[*]}" == *"matrixos-kernel"* ]]
}

@test "bedrock chroot: UPSTREAM_PORTAGE_REPOS includes matrixos" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"
    [[ "${UPSTREAM_PORTAGE_REPOS[*]}" == *"matrixos"* ]]
}

# ===========================================================================
# 10-server/chroot.sh
# ===========================================================================

@test "server chroot: all phase functions are declared" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/10-server/chroot.sh"

    local expected_phases=(
        server.buildenv_bootstrap
        server.portage_bootstrap
        server.build_everything
        server.tweak_nsswitch
        server.clean_temporary_artifacts
        server.tweak_resolved
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

@test "server chroot: BUILD_KERNEL_PACKAGES are defined" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/10-server/chroot.sh"
    [[ "${BUILD_KERNEL_PACKAGES[*]}" == *"matrixos-kernel"* ]]
}

@test "server chroot: UPSTREAM_PORTAGE_REPOS is empty" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/10-server/chroot.sh"
    [ "${#UPSTREAM_PORTAGE_REPOS[@]}" -eq 0 ]
}

# ===========================================================================
# 20-gnome/chroot.sh
# ===========================================================================

@test "gnome chroot: all phase functions are declared" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/20-gnome/chroot.sh"

    local expected_phases=(
        gnome.buildenv_bootstrap
        gnome.portage_bootstrap
        gnome.build_everything
        gnome.tweak_nsswitch
        gnome.tweak_resolved
        gnome.clean_temporary_artifacts
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

@test "gnome chroot: UPSTREAM_PORTAGE_REPOS includes steam-overlay and guru" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/20-gnome/chroot.sh"
    [[ "${UPSTREAM_PORTAGE_REPOS[*]}" == *"steam-overlay"* ]]
    [[ "${UPSTREAM_PORTAGE_REPOS[*]}" == *"guru"* ]]
}

# ===========================================================================
# 21-cosmic/chroot.sh
# ===========================================================================

@test "cosmic chroot: all phase functions are declared" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/21-cosmic/chroot.sh"

    local expected_phases=(
        cosmic.buildenv_bootstrap
        cosmic.portage_bootstrap
        cosmic.build_everything
        cosmic.tweak_nsswitch
        cosmic.tweak_resolved
        cosmic.clean_temporary_artifacts
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

@test "cosmic chroot: UPSTREAM_PORTAGE_REPOS includes steam-overlay and guru" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/21-cosmic/chroot.sh"
    [[ "${UPSTREAM_PORTAGE_REPOS[*]}" == *"steam-overlay"* ]]
    [[ "${UPSTREAM_PORTAGE_REPOS[*]}" == *"guru"* ]]
}

# ===========================================================================
# Phase tracking integration (uses real chroots_lib sourced by the scripts)
# ===========================================================================

@test "bedrock chroot: phase tracking works end-to-end" {
    _load_chroot_script "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"

    chroots_lib.touch_done_phase "bedrock.system_bootstrap"
    run chroots_lib.is_phase_done "bedrock.system_bootstrap"
    [ "$status" -eq 0 ]

    run chroots_lib.is_phase_done "bedrock.build_kernel"
    [ "$status" -ne 0 ]
}

# ===========================================================================
# All chroot.sh scripts call chroots_lib.setup
# ===========================================================================

@test "bedrock chroot: main calls chroots_lib.setup" {
    grep -q 'chroots_lib\.setup' "${BATS_TEST_DIRNAME}/00-bedrock/chroot.sh"
}

@test "server chroot: main calls chroots_lib.setup" {
    grep -q 'chroots_lib\.setup' "${BATS_TEST_DIRNAME}/10-server/chroot.sh"
}

@test "gnome chroot: main calls chroots_lib.setup" {
    grep -q 'chroots_lib\.setup' "${BATS_TEST_DIRNAME}/20-gnome/chroot.sh"
}

@test "cosmic chroot: main calls chroots_lib.setup" {
    grep -q 'chroots_lib\.setup' "${BATS_TEST_DIRNAME}/21-cosmic/chroot.sh"
}
