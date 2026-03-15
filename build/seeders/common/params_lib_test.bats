#!/usr/bin/env bats
# Unit tests for build/seeders/common/params_lib.sh

setup() {
    TEST_TMPDIR="$(mktemp -d)"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Default date stub (Wednesday of a week starting 2025-06-09).
    cat > "${STUB_BIN}/date" << 'DATEOF'
#!/bin/bash
if [[ "$*" == *"+%u"* ]]; then
    echo "3"
elif [[ "$*" == *"-d"* ]] && [[ "$*" == *"+%Y%m%d"* ]]; then
    echo "20250609"
elif [[ "$*" == *"+%Y%m%d"* ]]; then
    echo "20250611"
elif [[ "$*" == *"+%Y%m01"* ]]; then
    echo "20250601"
else
    echo "20250611"
fi
DATEOF
    chmod +x "${STUB_BIN}/date"

    # Source the library under test.
    source "${BATS_TEST_DIRNAME}/params_lib.sh"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# --- params_lib.get_chroot_date ---

@test "get_chroot_date defaults to weekly cadence" {
    unset SEEDER_DATE_CADENCE
    run params_lib.get_chroot_date
    [ "$status" -eq 0 ]
    [ "$output" = "20250609" ]
}

@test "get_chroot_date weekly cadence" {
    SEEDER_DATE_CADENCE="weekly"
    run params_lib.get_chroot_date
    [ "$status" -eq 0 ]
    [ "$output" = "20250609" ]
}

@test "get_chroot_date daily cadence" {
    SEEDER_DATE_CADENCE="daily"
    run params_lib.get_chroot_date
    [ "$status" -eq 0 ]
    [ "$output" = "20250611" ]
}

@test "get_chroot_date monthly cadence" {
    SEEDER_DATE_CADENCE="monthly"
    run params_lib.get_chroot_date
    [ "$status" -eq 0 ]
    [ "$output" = "20250601" ]
}

@test "get_chroot_date invalid cadence fails" {
    SEEDER_DATE_CADENCE="invalid"
    run params_lib.get_chroot_date
    [ "$status" -eq 1 ]
    [[ "$output" == *"invalid"* ]]
}

# --- params_lib.get_chroot_seeder_done_flag_file ---

@test "get_chroot_seeder_done_flag_file constructs correct path" {
    export SEEDER_DONE_FLAG_FILE_PREFIX="done-prefix"
    export SEEDERS_PHASES_STATE_DIR="/phases"

    run params_lib.get_chroot_seeder_done_flag_file "myseeder" "/tmp/chroots/bedrock-20250609"
    [ "$status" -eq 0 ]
    [ "$output" = "/tmp/chroots/bedrock-20250609/phases/done-prefix_myseeder" ]
}

@test "get_chroot_seeder_done_flag_file strips trailing slash from chroot_dir" {
    export SEEDER_DONE_FLAG_FILE_PREFIX="done"
    export SEEDERS_PHASES_STATE_DIR="/state"

    run params_lib.get_chroot_seeder_done_flag_file "test" "/tmp/chroots/dir/"
    [ "$status" -eq 0 ]
    [ "$output" = "/tmp/chroots/dir/state/done_test" ]
}

@test "get_chroot_seeder_done_flag_file fails without seeder name" {
    export SEEDER_DONE_FLAG_FILE_PREFIX="/done"
    export SEEDERS_PHASES_STATE_DIR="/state"

    run params_lib.get_chroot_seeder_done_flag_file "" "/tmp/chroots/dir"
    [ "$status" -eq 1 ]
}

@test "get_chroot_seeder_done_flag_file fails without chroot dir" {
    export SEEDER_DONE_FLAG_FILE_PREFIX="/done"
    export SEEDERS_PHASES_STATE_DIR="/state"

    run params_lib.get_chroot_seeder_done_flag_file "myseeder" ""
    [ "$status" -eq 1 ]
}

@test "get_chroot_seeder_done_flag_file fails without SEEDER_DONE_FLAG_FILE_PREFIX" {
    export SEEDER_DONE_FLAG_FILE_PREFIX=""
    export SEEDERS_PHASES_STATE_DIR="/state"

    run params_lib.get_chroot_seeder_done_flag_file "myseeder" "/tmp/dir"
    [ "$status" -eq 1 ]
}

@test "get_chroot_seeder_done_flag_file fails without SEEDERS_PHASES_STATE_DIR" {
    export SEEDER_DONE_FLAG_FILE_PREFIX="/done"
    export SEEDERS_PHASES_STATE_DIR=""

    run params_lib.get_chroot_seeder_done_flag_file "myseeder" "/tmp/dir"
    [ "$status" -eq 1 ]
}
