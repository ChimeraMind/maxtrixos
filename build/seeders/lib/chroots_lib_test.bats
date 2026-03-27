#!/usr/bin/env bats
# Unit tests for chroots_lib.sh using bats-core.

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    # Set the env vars the library expects.
    export SEEDERS_PHASES_STATE_DIR="${TEST_TMPDIR}/phases"
    export MATRIXOS_DEV_DIR="${TEST_TMPDIR}/matrixos"
    export DEFAULT_PRIVATE_GIT_REPO_PATH="${TEST_TMPDIR}/private"

    # Create minimal directory structure.
    mkdir -p "${MATRIXOS_DEV_DIR}/build/seeders"
    touch "${MATRIXOS_DEV_DIR}/.matrixos"

    # Stub bin dir for mocking external commands.
    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Source the library under test.
    source "${BATS_TEST_DIRNAME}/chroots_lib.sh"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# --- _get_phase_path / touch_done_phase / is_phase_done ---

@test "touch_done_phase creates phase file" {
    chroots_lib.touch_done_phase "my-phase"
    [ -f "${SEEDERS_PHASES_STATE_DIR}/my-phase.done" ]
}

@test "is_phase_done returns success after touch_done_phase" {
    chroots_lib.touch_done_phase "test-phase"
    run chroots_lib.is_phase_done "test-phase"
    [ "$status" -eq 0 ]
}

@test "is_phase_done returns failure when phase not touched" {
    run chroots_lib.is_phase_done "nonexistent-phase"
    [ "$status" -ne 0 ]
}

# --- package_list_path ---

@test "package_list_path returns correct path" {
    run chroots_lib.package_list_path "00-bedrock"
    [ "$status" -eq 0 ]
    [ "$output" = "${MATRIXOS_DEV_DIR}/build/seeders/00-bedrock/packages.conf" ]
}

@test "package_list_path returns correct path for another seeder" {
    run chroots_lib.package_list_path "10-server"
    [ "$status" -eq 0 ]
    [ "$output" = "${MATRIXOS_DEV_DIR}/build/seeders/10-server/packages.conf" ]
}

@test "package_list_path fails with empty seeder name" {
    run chroots_lib.package_list_path ""
    [ "$status" -eq 1 ]
}

# --- portage_confdir_path ---

@test "portage_confdir_path returns correct path" {
    run chroots_lib.portage_confdir_path "10-server"
    [ "$status" -eq 0 ]
    [ "$output" = "${MATRIXOS_DEV_DIR}/build/seeders/10-server/portage" ]
}

@test "portage_confdir_path fails with empty seeder name" {
    run chroots_lib.portage_confdir_path ""
    [ "$status" -eq 1 ]
}

# --- validate_package_list_path ---

@test "validate_package_list_path succeeds for existing file" {
    local f="${TEST_TMPDIR}/pkg.conf"
    touch "$f"
    run chroots_lib.validate_package_list_path "$f"
    [ "$status" -eq 0 ]
}

@test "validate_package_list_path fails for missing file" {
    run chroots_lib.validate_package_list_path "/nonexistent/path"
    [ "$status" -eq 1 ]
}

@test "validate_package_list_path fails for empty argument" {
    run chroots_lib.validate_package_list_path ""
    [ "$status" -eq 1 ]
}

# --- validate_portage_confdir_path ---

@test "validate_portage_confdir_path succeeds for existing dir" {
    mkdir -p "${TEST_TMPDIR}/portage"
    run chroots_lib.validate_portage_confdir_path "${TEST_TMPDIR}/portage"
    [ "$status" -eq 0 ]
}

@test "validate_portage_confdir_path fails for missing dir" {
    run chroots_lib.validate_portage_confdir_path "/nonexistent/dir"
    [ "$status" -eq 1 ]
}

@test "validate_portage_confdir_path fails for empty argument" {
    run chroots_lib.validate_portage_confdir_path ""
    [ "$status" -eq 1 ]
}

# --- validate_matrixos_git_repo ---

@test "validate_matrixos_git_repo succeeds when flag file exists" {
    run chroots_lib.validate_matrixos_git_repo
    [ "$status" -eq 0 ]
}

@test "validate_matrixos_git_repo fails when flag file missing" {
    rm "${MATRIXOS_DEV_DIR}/.matrixos"
    run chroots_lib.validate_matrixos_git_repo
    [ "$status" -eq 1 ]
}

# --- check_matrixos_private ---

@test "check_matrixos_private succeeds when dir exists" {
    mkdir -p "${TEST_TMPDIR}/priv"
    run chroots_lib.check_matrixos_private "${TEST_TMPDIR}/priv"
    [ "$status" -eq 0 ]
}

@test "check_matrixos_private fails when empty" {
    run chroots_lib.check_matrixos_private ""
    [ "$status" -eq 1 ]
}

@test "check_matrixos_private fails when dir missing" {
    run chroots_lib.check_matrixos_private "${TEST_TMPDIR}/surely-does-not-exist"
    [ "$status" -eq 1 ]
}

# --- store_portage_counter / get_stored_portage_counter ---

@test "store and get portage counter round-trips" {
    chroots_lib.store_portage_counter "42"
    run chroots_lib.get_stored_portage_counter
    [ "$status" -eq 0 ]
    [ "$output" = "42" ]
}

@test "get_stored_portage_counter returns -1 when no file exists" {
    run chroots_lib.get_stored_portage_counter
    [ "$status" -eq 0 ]
    [ "$output" = "-1" ]
}

@test "store_portage_counter fails with empty argument" {
    run chroots_lib.store_portage_counter ""
    [ "$status" -eq 1 ]
}

@test "store_portage_counter overwrites previous value" {
    chroots_lib.store_portage_counter "10"
    chroots_lib.store_portage_counter "99"
    run chroots_lib.get_stored_portage_counter
    [ "$status" -eq 0 ]
    [ "$output" = "99" ]
}

# --- try_get_emerge_jobs_flags ---

@test "try_get_emerge_jobs_flags returns --jobs and --load-average" {
    run chroots_lib.try_get_emerge_jobs_flags
    [ "$status" -eq 0 ]
    [[ "$output" == *"--jobs="* ]]
    [[ "$output" == *"--load-average="* ]]
}

# --- emerge_common_args ---

@test "emerge_common_args includes expected flags" {
    run chroots_lib.emerge_common_args
    [ "$status" -eq 0 ]
    [[ "$output" == *"--buildpkg"* ]]
    [[ "$output" == *"--usepkg"* ]]
    [[ "$output" == *"--verbose"* ]]
    [[ "$output" == *"--binpkg-respect-use=y"* ]]
    [[ "$output" == *"--quiet-build=y"* ]]
    [[ "$output" == *"--jobs="* ]]
}

# --- emerge_common_rebuild_args ---

@test "emerge_common_rebuild_args includes expected flags" {
    run chroots_lib.emerge_common_rebuild_args
    [ "$status" -eq 0 ]
    [[ "$output" == *"--quiet-build=y"* ]]
    [[ "$output" == *"--verbose"* ]]
    [[ "$output" == *"--jobs="* ]]
}

@test "emerge_common_rebuild_args does not include --buildpkg" {
    run chroots_lib.emerge_common_rebuild_args
    [ "$status" -eq 0 ]
    [[ "$output" != *"--buildpkg"* ]]
}

# --- default_clean_temporary_artifacts (with stubbed env-update) ---

@test "default_clean_temporary_artifacts removes expected dirs" {
    # Stub env-update.
    echo '#!/bin/bash
exit 0' > "${STUB_BIN}/env-update"
    chmod +x "${STUB_BIN}/env-update"

    # Create the dirs it should clean.
    mkdir -p /var/tmp/portage 2>/dev/null || skip "Cannot create /var/tmp/portage (not root)"
    mkdir -p /var/cache/revdep-rebuild 2>/dev/null || skip "Cannot create /var/cache/revdep-rebuild"

    run chroots_lib.default_clean_temporary_artifacts
    [ "$status" -eq 0 ]
}

# --- default_buildenv_bootstrap error paths (with stubs) ---

@test "default_buildenv_bootstrap fails with empty seeder name" {
    run chroots_lib.default_buildenv_bootstrap ""
    [ "$status" -eq 1 ]
}

# --- default_build_everything error paths ---

@test "default_build_everything fails with empty seeder name" {
    run chroots_lib.default_build_everything ""
    [ "$status" -eq 1 ]
}

# --- rebuild_before_portage_counter error path ---

@test "rebuild_before_portage_counter fails with empty argument" {
    run chroots_lib.rebuild_before_portage_counter ""
    [ "$status" -eq 1 ]
}

# --- generic_build with stubbed emerge ---

@test "generic_build calls emerge with common args" {
    # Stub emerge to just echo its args.
    cat > "${STUB_BIN}/emerge" << 'EOF'
#!/bin/bash
echo "EMERGE_CALLED: $*"
EOF
    chmod +x "${STUB_BIN}/emerge"

    # Stub env-update.
    echo '#!/bin/bash
exit 0' > "${STUB_BIN}/env-update"
    chmod +x "${STUB_BIN}/env-update"

    run chroots_lib.generic_build --newuse dev-libs/foo
    [ "$status" -eq 0 ]
    [[ "$output" == *"EMERGE_CALLED:"* ]]
    [[ "$output" == *"--buildpkg"* ]]
    [[ "$output" == *"--newuse"* ]]
    [[ "$output" == *"dev-libs/foo"* ]]
}

# --- generic_forced_rebuild with stubbed emerge ---

@test "generic_forced_rebuild calls emerge with rebuild args" {
    cat > "${STUB_BIN}/emerge" << 'EOF'
#!/bin/bash
echo "EMERGE_CALLED: $*"
EOF
    chmod +x "${STUB_BIN}/emerge"

    echo '#!/bin/bash
exit 0' > "${STUB_BIN}/env-update"
    chmod +x "${STUB_BIN}/env-update"

    run chroots_lib.generic_forced_rebuild --oneshot dev-libs/bar
    [ "$status" -eq 0 ]
    [[ "$output" == *"EMERGE_CALLED:"* ]]
    [[ "$output" == *"--oneshot"* ]]
    [[ "$output" == *"dev-libs/bar"* ]]
    # Should NOT contain --buildpkg (rebuild args, not common args).
    [[ "$output" != *"--buildpkg"* ]]
}

# --- setup_zombie_reaper ---

@test "setup_zombie_reaper installs CHLD trap" {
    run chroots_lib.setup_zombie_reaper
    [ "$status" -eq 0 ]
    [[ "$output" == *"Installing SIGCHLD zombie reaper"* ]]
}

# --- setup_cleanup ---

@test "setup_cleanup installs EXIT trap" {
    run chroots_lib.setup_cleanup
    [ "$status" -eq 0 ]
    [[ "$output" == *"Setting up EXIT trap"* ]]
}

# --- setup_cancellation ---

@test "setup_cancellation installs TERM/INT trap" {
    run chroots_lib.setup_cancellation
    [ "$status" -eq 0 ]
    [[ "$output" == *"Installing SIGTERM/SIGINT cancellation trap"* ]]
}

# --- _reap_zombies ---

@test "_reap_zombies does not error" {
    run chroots_lib._reap_zombies
    [ "$status" -eq 0 ]
}

# --- setup ---

@test "setup skips namespace traps when not PID 1" {
    # Stub mount-related commands to avoid real mounts.
    cat > "${STUB_BIN}/mountpoint" << 'EOF'
#!/bin/bash
# Pretend everything is already mounted.
exit 0
EOF
    chmod +x "${STUB_BIN}/mountpoint"

    cat > "${STUB_BIN}/readlink" << 'EOF'
#!/bin/bash
echo "/usr/bin/bash"
EOF
    chmod +x "${STUB_BIN}/readlink"

    run chroots_lib.setup
    [ "$status" -eq 0 ]
    # Since we're not PID 1, it should mention skipping.
    [[ "$output" == *"Not PID 1"* ]]
    [[ "$output" == *"Skipping namespace traps"* ]]
    # Should still dump mount info and PID 1 identity.
    [[ "$output" == *"Dump of /proc/self/mountinfo"* ]]
    [[ "$output" == *"PID 1 is"* ]]
}

# --- cleanup ---

@test "cleanup does not error" {
    run chroots_lib.cleanup
    [ "$status" -eq 0 ]
}

# --- default_portage_bootstrap with stubs ---

@test "default_portage_bootstrap enables and syncs repos" {
    cat > "${STUB_BIN}/eselect" << 'EOF'
#!/bin/bash
echo "ESELECT: $*"
EOF
    chmod +x "${STUB_BIN}/eselect"

    cat > "${STUB_BIN}/emaint" << 'EOF'
#!/bin/bash
echo "EMAINT: $*"
EOF
    chmod +x "${STUB_BIN}/emaint"

    run chroots_lib.default_portage_bootstrap "gentoo" "musl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ESELECT: repository enable gentoo"* ]]
    [[ "$output" == *"ESELECT: repository enable musl"* ]]
    [[ "$output" == *"EMAINT: --repo=gentoo sync"* ]]
    [[ "$output" == *"EMAINT: --repo=musl sync"* ]]
}

# --- clean_old_binpkgs ---

_stub_find_and_emaint() {
    export FIND_LOG="${TEST_TMPDIR}/find.log"
    export EMAINT_LOG="${TEST_TMPDIR}/emaint.log"
    cat > "${STUB_BIN}/find" << 'STUB'
#!/bin/bash
echo "$*" >> "$FIND_LOG"
STUB
    chmod +x "${STUB_BIN}/find"

    cat > "${STUB_BIN}/emaint" << 'STUB'
#!/bin/bash
echo "$*" >> "$EMAINT_LOG"
STUB
    chmod +x "${STUB_BIN}/emaint"
}

@test "clean_old_binpkgs invokes find for stale .tbz2, .gpkg.tar, .xpak files" {
    _stub_find_and_emaint

    run chroots_lib.clean_old_binpkgs
    [ "$status" -eq 0 ]

    # First find call: stale package deletion.
    local line1
    line1=$(sed -n '1p' "${FIND_LOG}")
    [[ "${line1}" == "/var/cache/binpkgs"* ]]
    [[ "${line1}" == *"-type f"* ]]
    [[ "${line1}" == *"*.tbz2"* ]]
    [[ "${line1}" == *"*.gpkg.tar"* ]]
    [[ "${line1}" == *"*.xpak"* ]]
    [[ "${line1}" == *"-atime +30"* ]]
    [[ "${line1}" == *"-print"* ]]
    [[ "${line1}" == *"-delete"* ]]
}

@test "clean_old_binpkgs prunes empty directories with -mindepth 1" {
    _stub_find_and_emaint

    run chroots_lib.clean_old_binpkgs
    [ "$status" -eq 0 ]

    # Second find call: empty directory pruning.
    local line2
    line2=$(sed -n '2p' "${FIND_LOG}")
    [[ "${line2}" == "/var/cache/binpkgs -mindepth 1 -type d -empty -delete" ]]
}

@test "clean_old_binpkgs calls emaint binhost --fix" {
    _stub_find_and_emaint

    run chroots_lib.clean_old_binpkgs
    [ "$status" -eq 0 ]

    local emaint_call
    emaint_call=$(cat "${EMAINT_LOG}")
    [ "${emaint_call}" = "binhost --fix" ]
}

@test "clean_old_binpkgs prints sweep and completion messages" {
    _stub_find_and_emaint

    run chroots_lib.clean_old_binpkgs
    [ "$status" -eq 0 ]
    [[ "$output" == *"Sweeping /var/cache/binpkgs for binpkgs unread in 30 days"* ]]
    [[ "$output" == *"Binary packages cache eviction complete"* ]]
}

# Note: Error propagation when find/emaint fails is handled by set -eu (script
# convention) and cannot be tested through bats' `run` which disables set -e.
