#!/usr/bin/env bats
# Unit tests for build/seeders/*/prepper.sh
#
# Each prepper.sh ends with `main "${@}"`, so we strip that line.
# The real preppers_lib.sh is sourced through MATRIXOS_DEV_DIR.

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
    export MATRIXOS_DEV_DIR="${REPO_ROOT}"

    # Env vars required by preppers_lib.sh.
    export SEEDER_LOCK_DIR="${TEST_TMPDIR}/locks"
    export SEEDER_LOCK_WAIT_SECS=5
    export SEEDER_GPG_KEYS_DIR="${TEST_TMPDIR}/gpg"
    export DOWNLOAD_DIR="${TEST_TMPDIR}/downloads"
    export PREPPERS_PHASES_STATE_DIR="/preppers-state"
    export SEEDER_BUILD_METADATA_FILE="build-metadata.txt"
    export SEEDER_CHROOT_NAME="test-chroot"
    export STAGE3_URL="https://example.com/stage3.tar.xz"
    export STAGE3_FILE=""
    export SEEDER_OVERLAY_GIT_REPO="https://example.com/overlay.git"
    export DEFAULT_PRIVATE_GIT_REPO_PATH="${TEST_TMPDIR}/private"
    export SEEDER_DONE_FLAG_FILE_PREFIX="/done-prefix"
    export SEEDERS_PHASES_STATE_DIR="/phases"
    export SEEDER_DATE_CADENCE="weekly"
    export CHROOT_DIR="${TEST_TMPDIR}/chroot"
    export CHROOT_RESUME=""

    mkdir -p "${TEST_TMPDIR}/private"
    mkdir -p "${TEST_TMPDIR}/downloads"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Stub external commands.
    for cmd in findmnt wget tar gpg gpgv rsync sleep; do
        cat > "${STUB_BIN}/${cmd}" << 'EOF'
#!/bin/bash
exit 0
EOF
        chmod +x "${STUB_BIN}/${cmd}"
    done

    # findmnt should return empty (no active mounts).
    cat > "${STUB_BIN}/findmnt" << 'EOF'
#!/bin/bash
echo ""
EOF
    chmod +x "${STUB_BIN}/findmnt"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# Helper: source a prepper.sh without executing main.
_load_prepper_script() {
    local script="${1}"
    local tmp="${TEST_TMPDIR}/_prepper_$$.sh"
    sed '/^main "\${@}"$/d' "${script}" > "${tmp}"
    source "${tmp}"
}

# ===========================================================================
# 00-bedrock/prepper.sh
# ===========================================================================

@test "bedrock prepper: all prep phase functions are declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/00-bedrock/prepper.sh"

    local expected_phases=(
        bedrock_prepper.prepare_rootfs
        bedrock_prepper.create_build_metadata
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

@test "bedrock prepper: download_latest_stage3 is declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/00-bedrock/prepper.sh"
    declare -F download_latest_stage3 >/dev/null 2>&1
}

@test "bedrock prepper: unpack_stage3_file is declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/00-bedrock/prepper.sh"
    declare -F unpack_stage3_file >/dev/null 2>&1
}

@test "bedrock prepper: prep phase tracking works" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/00-bedrock/prepper.sh"

    mkdir -p "${CHROOT_DIR}"
    preppers_lib.touch_done_prep_phase "${CHROOT_DIR}" "00-bedrock/bedrock_prepper.prepare_rootfs"
    run preppers_lib.is_prep_phase_done "${CHROOT_DIR}" "00-bedrock/bedrock_prepper.prepare_rootfs"
    [ "$status" -eq 0 ]

    run preppers_lib.is_prep_phase_done "${CHROOT_DIR}" "00-bedrock/bedrock_prepper.create_build_metadata"
    [ "$status" -ne 0 ]
}

# ===========================================================================
# 10-server/prepper.sh
# ===========================================================================

@test "server prepper: all prep phase functions are declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/10-server/prepper.sh"

    local expected_phases=(
        server_prepper.sync_from_bedrock
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

# ===========================================================================
# 20-gnome/prepper.sh
# ===========================================================================

@test "gnome prepper: all prep phase functions are declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/20-gnome/prepper.sh"

    local expected_phases=(
        gnome_prepper.sync_from_bedrock
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

# ===========================================================================
# 21-cosmic/prepper.sh
# ===========================================================================

@test "cosmic prepper: all prep phase functions are declared" {
    _load_prepper_script "${BATS_TEST_DIRNAME}/21-cosmic/prepper.sh"

    local expected_phases=(
        cosmic_prepper.sync_from_bedrock
    )

    for phase in "${expected_phases[@]}"; do
        declare -F "${phase}" >/dev/null 2>&1
    done
}

# ===========================================================================
# Structural consistency across all seeders
# ===========================================================================

@test "all preppers source preppers_lib.sh" {
    for seeder in 00-bedrock 10-server 20-gnome 21-cosmic; do
        local script="${BATS_TEST_DIRNAME}/${seeder}/prepper.sh"
        grep -q 'source.*preppers_lib.sh' "${script}"
    done
}

@test "all preppers call sanity_check_chroot_dir in main" {
    for seeder in 00-bedrock 10-server 20-gnome 21-cosmic; do
        local script="${BATS_TEST_DIRNAME}/${seeder}/prepper.sh"
        grep -q 'preppers_lib.sanity_check_chroot_dir' "${script}"
    done
}

@test "all preppers use phase tracking pattern" {
    for seeder in 00-bedrock 10-server 20-gnome 21-cosmic; do
        local script="${BATS_TEST_DIRNAME}/${seeder}/prepper.sh"
        grep -q 'preppers_lib.is_prep_phase_done' "${script}"
        grep -q 'preppers_lib.touch_done_prep_phase' "${script}"
    done
}
