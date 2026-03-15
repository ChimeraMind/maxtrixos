#!/usr/bin/env bats
# Unit tests for preppers_lib.sh using bats-core.

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    # Set env vars the library expects.
    export SEEDER_LOCK_DIR="${TEST_TMPDIR}/locks"
    export SEEDER_LOCK_WAIT_SECS=5
    export SEEDER_GPG_KEYS_DIR="${TEST_TMPDIR}/gpg"
    export DOWNLOAD_DIR="${TEST_TMPDIR}/downloads"
    export PREPPERS_PHASES_STATE_DIR="/preppers-state"
    export SEEDER_BUILD_METADATA_FILE="build-metadata.txt"
    export SEEDER_CHROOT_NAME="test-chroot"
    export STAGE3_URL="https://example.com/stage3.tar.xz"
    export STAGE3_FILE="stage3.tar.xz"

    # Stub bin dir for mocking external commands.
    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Stub findmnt (used by check_active_mounts / sanity checks).
    cat > "${STUB_BIN}/findmnt" << 'EOF'
#!/bin/bash
# Default stub: no mounts found.
echo ""
EOF
    chmod +x "${STUB_BIN}/findmnt"

    # Source the library under test.
    source "${BATS_TEST_DIRNAME}/preppers_lib.sh"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# --- check_dir_is_root ---

@test "check_dir_is_root succeeds for non-root dir" {
    local d="${TEST_TMPDIR}/chroot"
    mkdir -p "$d"
    run preppers_lib.check_dir_is_root "$d"
    [ "$status" -eq 0 ]
}

@test "check_dir_is_root fails with empty parameter" {
    run preppers_lib.check_dir_is_root ""
    [ "$status" -eq 1 ]
}

# --- check_active_mounts ---

@test "check_active_mounts succeeds when no mounts" {
    local d="${TEST_TMPDIR}/chroot"
    mkdir -p "$d"
    run preppers_lib.check_active_mounts "$d"
    [ "$status" -eq 0 ]
}

@test "check_active_mounts fails with empty parameter" {
    run preppers_lib.check_active_mounts ""
    [ "$status" -eq 1 ]
}

@test "check_active_mounts fails when mounts are active" {
    local d="${TEST_TMPDIR}/chroot"
    mkdir -p "$d"

    # Stub findmnt to return an active mount.
    cat > "${STUB_BIN}/findmnt" << EOF
#!/bin/bash
echo "${d}/dev"
EOF
    chmod +x "${STUB_BIN}/findmnt"

    run preppers_lib.check_active_mounts "$d" "test"
    [ "$status" -eq 1 ]
    [[ "$output" == *"Active mounts detected"* ]]
}

# --- check_hardlink_preservation ---

@test "check_hardlink_preservation fails with empty parameters" {
    run preppers_lib.check_hardlink_preservation "" ""
    [ "$status" -eq 1 ]
}

@test "check_hardlink_preservation fails with first param empty" {
    run preppers_lib.check_hardlink_preservation "" "/some/dir"
    [ "$status" -eq 1 ]
}

@test "check_hardlink_preservation fails with second param empty" {
    run preppers_lib.check_hardlink_preservation "/some/dir" ""
    [ "$status" -eq 1 ]
}

@test "check_hardlink_preservation returns 0 when no hardlinked files in source" {
    local src="${TEST_TMPDIR}/src"
    local dst="${TEST_TMPDIR}/dst"
    mkdir -p "$src" "$dst"
    echo "hello" > "$src/file1.txt"
    run preppers_lib.check_hardlink_preservation "$src" "$dst"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no hardlinked files found"* ]]
}

@test "check_hardlink_preservation succeeds when hardlinks preserved" {
    local src="${TEST_TMPDIR}/src"
    local dst="${TEST_TMPDIR}/dst"
    mkdir -p "$src" "$dst"

    # Create hardlinked files in source.
    echo "content" > "$src/file_a"
    ln "$src/file_a" "$src/file_b"

    # Copy preserving hardlinks.
    cp -a --preserve=links "$src/." "$dst/"

    run preppers_lib.check_hardlink_preservation "$src" "$dst"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SUCCESS"* ]]
}

@test "check_hardlink_preservation fails when hardlinks broken" {
    local src="${TEST_TMPDIR}/src"
    local dst="${TEST_TMPDIR}/dst"
    mkdir -p "$src" "$dst"

    # Create hardlinked files in source.
    echo "content" > "$src/file_a"
    ln "$src/file_a" "$src/file_b"

    # Copy WITHOUT preserving hardlinks (separate copies).
    cp "$src/file_a" "$dst/file_a"
    cp "$src/file_b" "$dst/file_b"

    run preppers_lib.check_hardlink_preservation "$src" "$dst"
    [ "$status" -eq 1 ]
    [[ "$output" == *"CRITICAL"* ]]
}

# --- create_temp_file ---

@test "create_temp_file creates a file in the specified dir" {
    run preppers_lib.create_temp_file "${TEST_TMPDIR}/temps" "myprefix"
    [ "$status" -eq 0 ]
    [ -f "$output" ]
    [[ "$output" == "${TEST_TMPDIR}/temps/myprefix."* ]]
}

@test "create_temp_file uses default prefix" {
    run preppers_lib.create_temp_file "${TEST_TMPDIR}/temps"
    [ "$status" -eq 0 ]
    [ -f "$output" ]
    [[ "$output" == "${TEST_TMPDIR}/temps/tmp."* ]]
}

@test "create_temp_file creates parent directory" {
    run preppers_lib.create_temp_file "${TEST_TMPDIR}/new/nested/dir" "test"
    [ "$status" -eq 0 ]
    [ -d "${TEST_TMPDIR}/new/nested/dir" ]
    [ -f "$output" ]
}

# --- seeder_chroot_dir_to_name ---

@test "seeder_chroot_dir_to_name extracts basename" {
    run preppers_lib.seeder_chroot_dir_to_name "/path/to/00-bedrock"
    [ "$status" -eq 0 ]
    [ "$output" = "00-bedrock" ]
}

@test "seeder_chroot_dir_to_name extracts basename with nested path" {
    run preppers_lib.seeder_chroot_dir_to_name "/var/lib/seeders/10-server"
    [ "$status" -eq 0 ]
    [ "$output" = "10-server" ]
}

@test "seeder_chroot_dir_to_name fails with empty parameter" {
    run preppers_lib.seeder_chroot_dir_to_name ""
    [ "$status" -eq 1 ]
}

# --- seeder_lock_dir ---

@test "seeder_lock_dir returns and creates lock dir" {
    run preppers_lib.seeder_lock_dir
    [ "$status" -eq 0 ]
    [ "$output" = "${SEEDER_LOCK_DIR}" ]
    [ -d "${SEEDER_LOCK_DIR}" ]
}

# --- seeder_lock_path ---

@test "seeder_lock_path returns correct path" {
    run preppers_lib.seeder_lock_path "00-bedrock"
    [ "$status" -eq 0 ]
    [ "$output" = "${SEEDER_LOCK_DIR}/00-bedrock.lock" ]
}

# --- execute_with_seeder_lock ---

@test "execute_with_seeder_lock runs function under lock" {
    my_test_func() {
        echo "CALLED with args: $*"
    }
    run preppers_lib.execute_with_seeder_lock "my_test_func" "test-seeder" "arg1" "arg2"
    [ "$status" -eq 0 ]
    [[ "$output" == *"CALLED with args: arg1 arg2"* ]]
    [[ "$output" == *"Lock for seeder test-seeder"* ]]
}

@test "execute_with_seeder_lock propagates function failure" {
    my_failing_func() {
        return 1
    }
    run preppers_lib.execute_with_seeder_lock "my_failing_func" "test-seeder"
    [ "$status" -eq 1 ]
}

# --- get_gpg_keychain_dir ---

@test "get_gpg_keychain_dir returns and creates dir" {
    run preppers_lib.get_gpg_keychain_dir
    [ "$status" -eq 0 ]
    [ "$output" = "${SEEDER_GPG_KEYS_DIR}" ]
    [ -d "${SEEDER_GPG_KEYS_DIR}" ]
}

# --- get_download_dir ---

@test "get_download_dir returns and creates dir" {
    run preppers_lib.get_download_dir
    [ "$status" -eq 0 ]
    [ "$output" = "${DOWNLOAD_DIR}" ]
    [ -d "${DOWNLOAD_DIR}" ]
}

# --- touch_done_prep_phase / is_prep_phase_done ---

@test "touch_done_prep_phase creates phase file" {
    local chroot_dir="${TEST_TMPDIR}/chroot"
    mkdir -p "$chroot_dir"
    preppers_lib.touch_done_prep_phase "$chroot_dir" "my-prep"
    [ -f "${chroot_dir}${PREPPERS_PHASES_STATE_DIR}/my-prep.done" ]
}

@test "touch_done_prep_phase fails with empty chroot_dir" {
    run preppers_lib.touch_done_prep_phase "" "phase"
    [ "$status" -eq 1 ]
}

@test "touch_done_prep_phase fails with empty prep_phase" {
    run preppers_lib.touch_done_prep_phase "/tmp/chroot" ""
    [ "$status" -eq 1 ]
}

@test "is_prep_phase_done returns success after touch" {
    local chroot_dir="${TEST_TMPDIR}/chroot"
    mkdir -p "$chroot_dir"
    preppers_lib.touch_done_prep_phase "$chroot_dir" "test-prep"
    run preppers_lib.is_prep_phase_done "$chroot_dir" "test-prep"
    [ "$status" -eq 0 ]
}

@test "is_prep_phase_done returns failure when not touched" {
    local chroot_dir="${TEST_TMPDIR}/chroot"
    mkdir -p "$chroot_dir"
    run preppers_lib.is_prep_phase_done "$chroot_dir" "nonexistent"
    [ "$status" -ne 0 ]
}

@test "is_prep_phase_done fails with empty chroot_dir" {
    run preppers_lib.is_prep_phase_done "" "phase"
    [ "$status" -eq 1 ]
}

@test "is_prep_phase_done fails with empty prep_phase" {
    run preppers_lib.is_prep_phase_done "/tmp/chroot" ""
    [ "$status" -eq 1 ]
}

# --- sanity_check_latest_bedrock ---

@test "sanity_check_latest_bedrock fails with empty parameter" {
    run preppers_lib.sanity_check_latest_bedrock ""
    [ "$status" -eq 1 ]
}

@test "sanity_check_latest_bedrock fails when dir missing" {
    run preppers_lib.sanity_check_latest_bedrock "/nonexistent"
    [ "$status" -eq 1 ]
}

@test "sanity_check_latest_bedrock fails when /dev is missing" {
    local bd="${TEST_TMPDIR}/bedrock"
    mkdir -p "$bd/proc" "$bd/sys"
    # /dev missing
    run preppers_lib.sanity_check_latest_bedrock "$bd"
    [ "$status" -eq 1 ]
}

@test "sanity_check_latest_bedrock succeeds with valid structure" {
    local bd="${TEST_TMPDIR}/bedrock"
    mkdir -p "$bd/dev" "$bd/proc" "$bd/sys"
    run preppers_lib.sanity_check_latest_bedrock "$bd"
    [ "$status" -eq 0 ]
}

# --- create_build_metadata_file ---

@test "create_build_metadata_file creates metadata with expected content" {
    local chroot_dir="${TEST_TMPDIR}/chroot"
    local bedrock_dir="${TEST_TMPDIR}/bedrocks/00-bedrock"
    mkdir -p "$chroot_dir" "$bedrock_dir"

    run preppers_lib.create_build_metadata_file "$chroot_dir" "$bedrock_dir"
    [ "$status" -eq 0 ]

    local metadata_file="${chroot_dir}/${SEEDER_BUILD_METADATA_FILE}"
    [ -f "$metadata_file" ]
    grep -q "BEDROCK_ORIGIN=00-bedrock" "$metadata_file"
    grep -q "SEED_NAME=${SEEDER_CHROOT_NAME}" "$metadata_file"
    grep -q "STAGE3_URL=${STAGE3_URL}" "$metadata_file"
    grep -q "STAGE3_FILE=${STAGE3_FILE}" "$metadata_file"
}

@test "create_build_metadata_file fails with empty chroot_dir" {
    run preppers_lib.create_build_metadata_file "" "/some/bedrock"
    [ "$status" -eq 1 ]
}

@test "create_build_metadata_file fails with empty bedrock_chroot_dir" {
    run preppers_lib.create_build_metadata_file "/some/chroot" ""
    [ "$status" -eq 1 ]
}

# --- _cp_reflink_copy ---

@test "_cp_reflink_copy copies source to destination" {
    local src="${TEST_TMPDIR}/src"
    local dst="${TEST_TMPDIR}/dst"
    mkdir -p "$src"
    echo "test content" > "$src/file.txt"

    run preppers_lib._cp_reflink_copy "$src" "$dst"
    [ "$status" -eq 0 ]
    [ -f "$dst/file.txt" ]
    [ "$(cat "$dst/file.txt")" = "test content" ]
}

@test "_cp_reflink_copy removes existing destination" {
    local src="${TEST_TMPDIR}/src"
    local dst="${TEST_TMPDIR}/dst"
    mkdir -p "$src" "$dst"
    echo "old" > "$dst/old_file.txt"
    echo "new" > "$src/new_file.txt"

    run preppers_lib._cp_reflink_copy "$src" "$dst"
    [ "$status" -eq 0 ]
    [ -f "$dst/new_file.txt" ]
    [ ! -f "$dst/old_file.txt" ]
}

# --- sanity_check_chroot_dir (non-resume, fresh dir) ---

@test "sanity_check_chroot_dir creates missing dir" {
    local d="${TEST_TMPDIR}/new-chroot"
    run preppers_lib.sanity_check_chroot_dir "$d" "" "test"
    [ "$status" -eq 0 ]
    [ -d "$d" ]
}

@test "sanity_check_chroot_dir fails with empty parameter" {
    run preppers_lib.sanity_check_chroot_dir "" "" "test"
    [ "$status" -eq 1 ]
}

@test "sanity_check_chroot_dir fails when path is a file not dir" {
    local f="${TEST_TMPDIR}/not-a-dir"
    touch "$f"
    run preppers_lib.sanity_check_chroot_dir "$f" "" "test"
    [ "$status" -eq 1 ]
    [[ "$output" == *"is not a directory"* ]]
}

@test "sanity_check_chroot_dir resume succeeds with functional rootfs" {
    local d="${TEST_TMPDIR}/chroot"
    mkdir -p "$d/bin"
    echo '#!/bin/sh' > "$d/bin/sh" && chmod +x "$d/bin/sh"
    echo '#!/bin/sh' > "$d/bin/ls" && chmod +x "$d/bin/ls"
    run preppers_lib.sanity_check_chroot_dir "$d" "yes" "test"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Attempting to resume"* ]]
}

@test "sanity_check_chroot_dir resume with non-functional rootfs is detected" {
    local d="${TEST_TMPDIR}/chroot-nonfunc"
    mkdir -p "$d"

    # Stub sleep to be instant.
    echo '#!/bin/bash
exit 0' > "${STUB_BIN}/sleep"
    chmod +x "${STUB_BIN}/sleep"

    # No /bin/sh or /bin/ls => not functional. Function will detect this
    # and attempt _move_chroot_dir_away (mktemp + mv).
    run preppers_lib.sanity_check_chroot_dir "$d" "yes" "test"
    [[ "$output" == *"NOT functional"* ]]
}

# --- gpg_verify_file (with stubbed gpg) ---

@test "gpg_verify_file calls gpg with correct args" {
    cat > "${STUB_BIN}/gpg" << 'EOF'
#!/bin/bash
echo "GPG: $*"
EOF
    chmod +x "${STUB_BIN}/gpg"

    run preppers_lib.gpg_verify_file "/some/file.tar"
    [ "$status" -eq 0 ]
    [[ "$output" == *"--verify"* ]]
    [[ "$output" == *"/some/file.tar.asc"* ]]
    [[ "$output" == *"/some/file.tar"* ]]
}

# --- gpg_verify_embedded_signature_file (with stubbed gpg) ---

@test "gpg_verify_embedded_signature_file calls gpg with correct args" {
    cat > "${STUB_BIN}/gpg" << 'EOF'
#!/bin/bash
echo "GPG: $*"
EOF
    chmod +x "${STUB_BIN}/gpg"

    run preppers_lib.gpg_verify_embedded_signature_file "/some/signed-file"
    [ "$status" -eq 0 ]
    [[ "$output" == *"--verify"* ]]
    [[ "$output" == *"/some/signed-file"* ]]
}

# --- _rsync_from_bedrock (with stubs) ---

@test "_rsync_from_bedrock fails with empty parameters" {
    run preppers_lib._rsync_from_bedrock "" ""
    [ "$status" -eq 1 ]
}

@test "rsync_from_bedrock fails with empty parameters" {
    run preppers_lib.rsync_from_bedrock "" ""
    [ "$status" -eq 1 ]
}
