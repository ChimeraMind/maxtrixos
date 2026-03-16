#!/usr/bin/env bats
# Unit tests for build/seeders/init.sh
#
# init.sh ends with `main "${@}"`, so we strip that line and source the
# remaining function definitions.  External commands (mount, mountpoint,
# chroot, mkdir) are stubbed to avoid requiring real privileges.

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"

    # Env vars expected by init.sh.
    export SEEDER_DISTFILES_DIR="${TEST_TMPDIR}/distfiles-host"
    export SEEDER_BINPKGS_DIR="${TEST_TMPDIR}/binpkgs-host"
    export SEEDER_PRIVATE_GIT_REPO_PATH="${TEST_TMPDIR}/private-host"
    export DEFAULT_PRIVATE_GIT_REPO_PATH="/var/lib/private-git"
    export RUNNER_TYPE="seeder"

    CHROOT_DIR="${TEST_TMPDIR}/chroot"
    mkdir -p "${CHROOT_DIR}"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Record file: stubs append their invocations here so tests can inspect.
    CALL_LOG="${TEST_TMPDIR}/calls.log"
    export CALL_LOG
    touch "${CALL_LOG}"

    # --- stubs ---

    # mountpoint: default to "not a mountpoint" (exit 1).
    cat > "${STUB_BIN}/mountpoint" << 'EOF'
#!/bin/bash
# If STUB_MOUNTPOINTS contains the queried path, report mounted.
for mp in ${STUB_MOUNTPOINTS:-}; do
    if [ "$2" = "${mp}" ]; then
        exit 0
    fi
done
exit 1
EOF
    chmod +x "${STUB_BIN}/mountpoint"

    # mount: record invocation and succeed.
    cat > "${STUB_BIN}/mount" << 'EOF'
#!/bin/bash
echo "mount $*" >> "${CALL_LOG}"
exit 0
EOF
    chmod +x "${STUB_BIN}/mount"

    # chroot: record invocation and succeed (used only via exec in main).
    cat > "${STUB_BIN}/chroot" << 'EOF'
#!/bin/bash
echo "chroot $*" >> "${CALL_LOG}"
exit 0
EOF
    chmod +x "${STUB_BIN}/chroot"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# Helper: source init.sh without executing main.
_load_init_script() {
    local tmp="${TEST_TMPDIR}/_init_$$.sh"
    sed '/^main "\${@}"$/d' "${BATS_TEST_DIRNAME}/init.sh" > "${tmp}"
    source "${tmp}"
}

# ===========================================================================
# setup_chroot_env – directory creation
# ===========================================================================

@test "init: setup_chroot_env creates host distfiles directory" {
    _load_init_script
    # Ensure host dir does not exist yet.
    rmdir "${SEEDER_DISTFILES_DIR}" 2>/dev/null || true
    [ ! -d "${SEEDER_DISTFILES_DIR}" ]

    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${SEEDER_DISTFILES_DIR}" ]
}

@test "init: setup_chroot_env creates chroot distfiles directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/var/cache/distfiles" ]
}

@test "init: setup_chroot_env creates host binpkgs directory" {
    _load_init_script
    rmdir "${SEEDER_BINPKGS_DIR}" 2>/dev/null || true
    [ ! -d "${SEEDER_BINPKGS_DIR}" ]

    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${SEEDER_BINPKGS_DIR}" ]
}

@test "init: setup_chroot_env creates chroot binpkgs directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/var/cache/binpkgs" ]
}

@test "init: setup_chroot_env creates chroot private git directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/${DEFAULT_PRIVATE_GIT_REPO_PATH#/}" ]
}

@test "init: setup_chroot_env creates chroot sys directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/sys" ]
}

@test "init: setup_chroot_env creates chroot dev directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/dev" ]
}

@test "init: setup_chroot_env creates chroot run/lock directory" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"
    [ -d "${CHROOT_DIR}/run/lock" ]
}

# ===========================================================================
# setup_chroot_env – bind mounts when not yet mounted
# ===========================================================================

@test "init: setup_chroot_env bind-mounts distfiles" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    grep -q "mount --bind ${SEEDER_DISTFILES_DIR} ${CHROOT_DIR}/var/cache/distfiles" "${CALL_LOG}"
    grep -q "mount --make-private ${CHROOT_DIR}/var/cache/distfiles" "${CALL_LOG}"
}

@test "init: setup_chroot_env bind-mounts binpkgs" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    grep -q "mount --bind ${SEEDER_BINPKGS_DIR} ${CHROOT_DIR}/var/cache/binpkgs" "${CALL_LOG}"
    grep -q "mount --make-private ${CHROOT_DIR}/var/cache/binpkgs" "${CALL_LOG}"
}

@test "init: setup_chroot_env bind-mounts private git repo" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    local expected_dst="${CHROOT_DIR}/${DEFAULT_PRIVATE_GIT_REPO_PATH#/}"
    grep -q "mount --bind ${SEEDER_PRIVATE_GIT_REPO_PATH} ${expected_dst}" "${CALL_LOG}"
    grep -q "mount --make-private ${expected_dst}" "${CALL_LOG}"
}

@test "init: setup_chroot_env mounts sysfs" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    grep -q "mount -t sysfs sys ${CHROOT_DIR}/sys" "${CALL_LOG}"
}

@test "init: setup_chroot_env mounts devtmpfs, devpts, and devshm" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    grep -q "mount -t devtmpfs devtmpfs ${CHROOT_DIR}/dev" "${CALL_LOG}"
    grep -q "mount -t devpts devpts ${CHROOT_DIR}/dev/pts" "${CALL_LOG}"
    grep -q "mount -t tmpfs devshm ${CHROOT_DIR}/dev/shm" "${CALL_LOG}"
}

@test "init: setup_chroot_env mounts run/lock tmpfs" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}"

    grep -q "mount -t tmpfs run ${CHROOT_DIR}/run/lock" "${CALL_LOG}"
}

# ===========================================================================
# setup_chroot_env – skips mounts when already mounted
# ===========================================================================

@test "init: setup_chroot_env skips distfiles mount when already mounted" {
    _load_init_script

    export STUB_MOUNTPOINTS="${CHROOT_DIR}/var/cache/distfiles"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted, skipping bind mount"* ]]
    # distfiles mount should not appear, but binpkgs and private git should.
    ! grep -q "mount --bind ${SEEDER_DISTFILES_DIR}" "${CALL_LOG}"
    grep -q "mount --bind ${SEEDER_BINPKGS_DIR}" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips binpkgs mount when already mounted" {
    _load_init_script

    export STUB_MOUNTPOINTS="${CHROOT_DIR}/var/cache/binpkgs"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted, skipping bind mount"* ]]
    ! grep -q "mount --bind ${SEEDER_BINPKGS_DIR}" "${CALL_LOG}"
    grep -q "mount --bind ${SEEDER_DISTFILES_DIR}" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips private git mount when already mounted" {
    _load_init_script

    local expected_dst="${CHROOT_DIR}/${DEFAULT_PRIVATE_GIT_REPO_PATH#/}"
    export STUB_MOUNTPOINTS="${expected_dst}"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted, skipping bind mount"* ]]
    ! grep -q "mount --bind ${SEEDER_PRIVATE_GIT_REPO_PATH}" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips sysfs mount when already mounted" {
    _load_init_script

    export STUB_MOUNTPOINTS="${CHROOT_DIR}/sys"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted on the host, skipping"* ]]
    ! grep -q "mount -t sysfs" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips dev mount when already mounted" {
    _load_init_script

    export STUB_MOUNTPOINTS="${CHROOT_DIR}/dev"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted on the host, skipping"* ]]
    ! grep -q "mount -t devtmpfs" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips run/lock mount when already mounted" {
    _load_init_script

    export STUB_MOUNTPOINTS="${CHROOT_DIR}/run/lock"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    [[ "$output" == *"already mounted on the host, skipping"* ]]
    ! grep -q "mount -t tmpfs run" "${CALL_LOG}"
}

@test "init: setup_chroot_env skips all mounts when all already mounted" {
    _load_init_script

    local expected_dst="${CHROOT_DIR}/${DEFAULT_PRIVATE_GIT_REPO_PATH#/}"
    export STUB_MOUNTPOINTS="${CHROOT_DIR}/var/cache/distfiles ${CHROOT_DIR}/var/cache/binpkgs ${expected_dst} ${CHROOT_DIR}/sys ${CHROOT_DIR}/dev ${CHROOT_DIR}/run/lock"
    run setup_chroot_env "${CHROOT_DIR}"

    [ "$status" -eq 0 ]
    # No mounts should have been performed at all.
    ! grep -q "^mount " "${CALL_LOG}"
}

# ===========================================================================
# setup_chroot_env – chroot_dir trailing slash handling
# ===========================================================================

@test "init: setup_chroot_env strips trailing slash from chroot_dir" {
    _load_init_script
    setup_chroot_env "${CHROOT_DIR}/"

    # Paths in call log should not contain double slashes.
    ! grep -q '//' "${CALL_LOG}"
}

# ===========================================================================
# main – argument handling and exec
# ===========================================================================

@test "init: main prints startup message to stderr" {
    _load_init_script

    # Override exec to prevent replacing the test process.
    exec() { "${@}"; }

    run main "${CHROOT_DIR}" "/seed.sh" "--flag"
    [[ "${output}" == *"Starting chroot()"* ]]
    [[ "${output}" == *"chroot_dir=${CHROOT_DIR}"* ]]
    [[ "${output}" == *"target_exec=/seed.sh"* ]]
    [[ "${output}" == *"args=--flag"* ]]
}

@test "init: main invokes chroot with correct arguments" {
    _load_init_script

    exec() { "${@}"; }

    main "${CHROOT_DIR}" "/seed.sh" "--flag" "extra"

    grep -q "chroot ${CHROOT_DIR} /seed.sh --flag extra" "${CALL_LOG}"
}

@test "init: main passes no extra args when none given" {
    _load_init_script

    exec() { "${@}"; }

    main "${CHROOT_DIR}" "/seed.sh"

    grep -q "chroot ${CHROOT_DIR} /seed.sh" "${CALL_LOG}"
}

# ===========================================================================
# Structural checks
# ===========================================================================

@test "init: script starts with set -eu" {
    grep -q '^set -eu' "${BATS_TEST_DIRNAME}/init.sh"
}

@test "init: script ends with main call" {
    tail -n 1 "${BATS_TEST_DIRNAME}/init.sh" | grep -q '^main "\${@}"$'
}
