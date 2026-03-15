#!/usr/bin/env bats
# Unit tests for dev/vector_builder.sh

setup() {
    TEST_TMPDIR="$(mktemp -d)"
    export MATRIXOS_DEV_DIR="${TEST_TMPDIR}/matrixos"
    mkdir -p "${MATRIXOS_DEV_DIR}/dev"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Load functions without running main.
    local tmp="${TEST_TMPDIR}/_script.sh"
    sed '/^main "\${@}"$/d' "${BATS_TEST_DIRNAME}/vector_builder.sh" > "${tmp}"
    source "${tmp}"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# ---------------------------------------------------------------------------
# parse_args - boolean flags
# ---------------------------------------------------------------------------

@test "parse_args: -fr sets ARG_FORCE_RELEASE" {
    parse_args -fr
    [ "${ARG_FORCE_RELEASE}" = "1" ]
}

@test "parse_args: --force-release sets ARG_FORCE_RELEASE" {
    parse_args --force-release
    [ "${ARG_FORCE_RELEASE}" = "1" ]
}

@test "parse_args: -oi sets ARG_ONLY_IMAGES" {
    parse_args -oi
    [ "${ARG_ONLY_IMAGES}" = "1" ]
}

@test "parse_args: --only-images sets ARG_ONLY_IMAGES" {
    parse_args --only-images
    [ "${ARG_ONLY_IMAGES}" = "1" ]
}

@test "parse_args: -fi sets ARG_FORCE_IMAGES" {
    parse_args -fi
    [ "${ARG_FORCE_IMAGES}" = "1" ]
}

@test "parse_args: -si sets ARG_SKIP_IMAGES" {
    parse_args -si
    [ "${ARG_SKIP_IMAGES}" = "1" ]
}

@test "parse_args: -bs sets ARG_ON_BUILD_SERVER" {
    parse_args -bs
    [ "${ARG_ON_BUILD_SERVER}" = "1" ]
}

@test "parse_args: -rs sets ARG_RESUME_SEEDERS" {
    parse_args -rs
    [ "${ARG_RESUME_SEEDERS}" = "1" ]
}

@test "parse_args: -dj clears ARG_RUN_JANITOR" {
    parse_args -dj
    [ -z "${ARG_RUN_JANITOR}" ]
}

@test "parse_args: --disable-send-mail clears ARG_SEND_MAIL" {
    parse_args --disable-send-mail
    [ -z "${ARG_SEND_MAIL}" ]
}

@test "parse_args: -no-sm clears ARG_SEND_MAIL" {
    parse_args -no-sm
    [ -z "${ARG_SEND_MAIL}" ]
}

# ---------------------------------------------------------------------------
# parse_args - valued flags (space form)
# ---------------------------------------------------------------------------

@test "parse_args: -bn sets ARG_BUILD_NAME (space)" {
    parse_args -bn "my test build"
    [ "${ARG_BUILD_NAME}" = "my test build" ]
}

@test "parse_args: -bi sets ARG_BUILD_ID (space)" {
    parse_args -bi myid
    [ "${ARG_BUILD_ID}" = "myid" ]
}

@test "parse_args: -cp sets ARG_CDN_PUSHER (space)" {
    parse_args -cp /usr/local/bin/push
    [ "${ARG_CDN_PUSHER}" = "/usr/local/bin/push" ]
}

# ---------------------------------------------------------------------------
# parse_args - valued flags (= form)
# ---------------------------------------------------------------------------

@test "parse_args: --build-name= sets ARG_BUILD_NAME" {
    parse_args --build-name=custom
    [ "${ARG_BUILD_NAME}" = "custom" ]
}

@test "parse_args: --build-id= sets ARG_BUILD_ID" {
    parse_args --build-id=testid
    [ "${ARG_BUILD_ID}" = "testid" ]
}

@test "parse_args: --cdn-pusher= sets ARG_CDN_PUSHER" {
    parse_args --cdn-pusher=/tmp/pusher
    [ "${ARG_CDN_PUSHER}" = "/tmp/pusher" ]
}

# ---------------------------------------------------------------------------
# parse_args - seeder selection
# ---------------------------------------------------------------------------

@test "parse_args: -s appends to ARG_SEEDER_ARGS and ARG_RELEASER_ARGS (space)" {
    parse_args -s "00-bedrock,10-server"
    [[ "${ARG_SEEDER_ARGS[*]}" == *"--skip-seeders=00-bedrock,10-server"* ]]
    [[ "${ARG_RELEASER_ARGS[*]}" == *"--skip-seeders=00-bedrock,10-server"* ]]
}

@test "parse_args: --skip-seeders= appends to ARG_SEEDER_ARGS" {
    parse_args --skip-seeders=00-bedrock
    [[ "${ARG_SEEDER_ARGS[*]}" == *"--skip-seeders=00-bedrock"* ]]
}

@test "parse_args: -o appends to ARG_SEEDER_ARGS (space)" {
    parse_args -o "00-bedrock"
    [[ "${ARG_SEEDER_ARGS[*]}" == *"--only-seeders=00-bedrock"* ]]
    [[ "${ARG_RELEASER_ARGS[*]}" == *"--only-seeders=00-bedrock"* ]]
}

@test "parse_args: --only-seeders= appends to ARG_SEEDER_ARGS" {
    parse_args --only-seeders=20-gnome
    [[ "${ARG_SEEDER_ARGS[*]}" == *"--only-seeders=20-gnome"* ]]
}

# ---------------------------------------------------------------------------
# parse_args - help & errors
# ---------------------------------------------------------------------------

@test "parse_args: -h exits 0 and shows usage" {
    run parse_args -h
    [ "$status" -eq 0 ]
    [[ "$output" == *"release - matrixOS"* ]]
    [[ "$output" == *"--force-release"* ]]
}

@test "parse_args: --help exits 0" {
    run parse_args --help
    [ "$status" -eq 0 ]
}

@test "parse_args: unknown flag returns 1" {
    run parse_args --nonexistent-flag
    [ "$status" -eq 1 ]
    [[ "$output" == *"Unknown argument"* ]]
}

@test "parse_args: positional args collected" {
    parse_args some-positional
    [[ "${ARG_POSITIONALS[*]}" == *"some-positional"* ]]
}

# ---------------------------------------------------------------------------
# parse_args - defaults
# ---------------------------------------------------------------------------

@test "parse_args: defaults are sane" {
    parse_args
    [ -z "${ARG_FORCE_RELEASE}" ]
    [ -z "${ARG_ONLY_IMAGES}" ]
    [ -z "${ARG_FORCE_IMAGES}" ]
    [ -z "${ARG_SKIP_IMAGES}" ]
    [ -z "${ARG_ON_BUILD_SERVER}" ]
    [ -z "${ARG_RESUME_SEEDERS}" ]
    [ "${ARG_BUILD_NAME}" = "matrixOS weekly" ]
    [ "${ARG_BUILD_ID}" = "weekly" ]
    [ "${ARG_RUN_JANITOR}" = "1" ]
    [ "${ARG_SEND_MAIL}" = "1" ]
    [ -z "${ARG_CDN_PUSHER}" ]
}

@test "parse_args: multiple flags combine correctly" {
    parse_args -fr -oi -si -dj -no-sm -bi testid
    [ "${ARG_FORCE_RELEASE}" = "1" ]
    [ "${ARG_ONLY_IMAGES}" = "1" ]
    [ "${ARG_SKIP_IMAGES}" = "1" ]
    [ -z "${ARG_RUN_JANITOR}" ]
    [ -z "${ARG_SEND_MAIL}" ]
    [ "${ARG_BUILD_ID}" = "testid" ]
}

# ---------------------------------------------------------------------------
# Flag helper functions
# ---------------------------------------------------------------------------

@test "_only_images_flag succeeds when set" {
    ARG_ONLY_IMAGES=1
    _only_images_flag
}

@test "_only_images_flag fails when empty" {
    ARG_ONLY_IMAGES=
    ! _only_images_flag
}

@test "_force_release_flag succeeds when set" {
    ARG_FORCE_RELEASE=1
    _force_release_flag
}

@test "_force_release_flag fails when empty" {
    ARG_FORCE_RELEASE=
    ! _force_release_flag
}

@test "_force_images_flag succeeds when set" {
    ARG_FORCE_IMAGES=1
    _force_images_flag
}

@test "_skip_images_flag succeeds when set" {
    ARG_SKIP_IMAGES=1
    _skip_images_flag
}

@test "_on_build_server_flag succeeds when set" {
    ARG_ON_BUILD_SERVER=1
    _on_build_server_flag
}

@test "_resume_seeders_flag succeeds when set" {
    ARG_RESUME_SEEDERS=1
    _resume_seeders_flag
}

@test "_run_janitor_flag succeeds when set" {
    ARG_RUN_JANITOR=1
    _run_janitor_flag
}

@test "_run_janitor_flag fails when empty" {
    ARG_RUN_JANITOR=
    ! _run_janitor_flag
}

@test "_cdn_pusher_flag echoes the value" {
    ARG_CDN_PUSHER="/usr/bin/pusher"
    run _cdn_pusher_flag
    [ "$output" = "/usr/bin/pusher" ]
}

@test "_build_id_flag echoes the value" {
    ARG_BUILD_ID="testid"
    run _build_id_flag
    [ "$output" = "testid" ]
}

@test "_build_name_flag echoes the value" {
    ARG_BUILD_NAME="My Build"
    run _build_name_flag
    [ "$output" = "My Build" ]
}

# ---------------------------------------------------------------------------
# finish
# ---------------------------------------------------------------------------

@test "finish cleans up temp files" {
    BUILT_SEEDERS_FILE="${TEST_TMPDIR}/seeds.tmp"
    BUILT_RELEASES_FILE="${TEST_TMPDIR}/releases.tmp"
    touch "${BUILT_SEEDERS_FILE}" "${BUILT_RELEASES_FILE}"

    ARG_HELP=""
    ARG_SEND_MAIL=""
    ARG_BUILD_NAME="test"

    # Stub id for mail_dest
    cat > "${STUB_BIN}/id" << 'EOF'
#!/bin/bash
echo "testuser"
EOF
    chmod +x "${STUB_BIN}/id"

    finish
    [ ! -f "${BUILT_SEEDERS_FILE}" ]
    [ ! -f "${BUILT_RELEASES_FILE}" ]
}

@test "finish skips cleanup when ARG_HELP is set" {
    BUILT_SEEDERS_FILE="${TEST_TMPDIR}/seeds.tmp"
    BUILT_RELEASES_FILE="${TEST_TMPDIR}/releases.tmp"
    touch "${BUILT_SEEDERS_FILE}" "${BUILT_RELEASES_FILE}"

    ARG_HELP=1

    finish
    # Files should still exist since the function returns early
    [ -f "${BUILT_SEEDERS_FILE}" ]
    [ -f "${BUILT_RELEASES_FILE}" ]
}

@test "finish handles empty BUILT_SEEDERS_FILE and BUILT_RELEASES_FILE" {
    BUILT_SEEDERS_FILE=""
    BUILT_RELEASES_FILE=""
    LOGFILE=""
    ARG_HELP=""
    ARG_SEND_MAIL=""
    ARG_BUILD_NAME="test"

    cat > "${STUB_BIN}/id" << 'EOF'
#!/bin/bash
echo "testuser"
EOF
    chmod +x "${STUB_BIN}/id"

    # The [[ -n "" ]] && ... pattern returns 1 when var is empty.
    # In production this runs as a trap handler where set -e is inactive.
    run finish
    # Status is 1 because the last [[ -n "" ]] && rm returns 1.
    [ "$status" -eq 1 ]
}
