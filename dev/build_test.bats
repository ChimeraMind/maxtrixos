#!/usr/bin/env bats
# Unit tests for dev/build.sh

setup() {
    TEST_TMPDIR="$(mktemp -d)"
    export MATRIXOS_DEV_DIR="${TEST_TMPDIR}/matrixos"
    mkdir -p "${MATRIXOS_DEV_DIR}/dev"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Stub sleep to avoid delays.
    cat > "${STUB_BIN}/sleep" << 'EOF'
#!/bin/bash
exit 0
EOF
    chmod +x "${STUB_BIN}/sleep"

    # Load functions from build.sh without running main.
    local tmp="${TEST_TMPDIR}/_script.sh"
    sed '/^main "\${@}"$/d' "${BATS_TEST_DIRNAME}/build.sh" > "${tmp}"
    source "${tmp}"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# --- _is_help_arg ---

@test "_is_help_arg returns 0 for -h" {
    run _is_help_arg "-h"
    [ "$status" -eq 0 ]
}

@test "_is_help_arg returns 0 for --help" {
    run _is_help_arg "--help"
    [ "$status" -eq 0 ]
}

@test "_is_help_arg returns 1 for other arg" {
    run _is_help_arg "--foo"
    [ "$status" -eq 1 ]
}

@test "_is_help_arg returns 1 for empty arg" {
    run _is_help_arg ""
    [ "$status" -eq 1 ]
}

@test "_is_help_arg returns 1 for no arg" {
    run _is_help_arg
    [ "$status" -eq 1 ]
}

# --- _is_help_in_args ---

@test "_is_help_in_args returns 0 when -h present" {
    run _is_help_in_args "foo" "-h" "bar"
    [ "$status" -eq 0 ]
}

@test "_is_help_in_args returns 0 when --help present" {
    run _is_help_in_args "--help"
    [ "$status" -eq 0 ]
}

@test "_is_help_in_args returns 1 with no help flags" {
    run _is_help_in_args "foo" "bar"
    [ "$status" -eq 1 ]
}

@test "_is_help_in_args returns 1 with no args" {
    run _is_help_in_args
    [ "$status" -eq 1 ]
}

# --- _root_privs ---

@test "_root_privs succeeds when uid is 0" {
    cat > "${STUB_BIN}/id" << 'EOF'
#!/bin/bash
echo "0"
EOF
    chmod +x "${STUB_BIN}/id"
    run _root_privs
    [ "$status" -eq 0 ]
}

@test "_root_privs fails when uid is non-zero" {
    cat > "${STUB_BIN}/id" << 'EOF'
#!/bin/bash
echo "1000"
EOF
    chmod +x "${STUB_BIN}/id"
    run _root_privs
    [ "$status" -eq 1 ]
    [[ "$output" == *"as root"* ]]
}

# --- _print_build_warning ---

@test "_print_build_warning prints warning text" {
    run _print_build_warning
    [ "$status" -eq 0 ]
    [[ "$output" == *"ATTENTION PLEASE"* ]]
    [[ "$output" == *"matrixOS.GitRepo"* ]]
    [[ "$output" == *"conf/matrixos.conf"* ]]
}

# --- main ---

@test "main passes --on-build-server --disable-send-mail to vector_builder.sh" {
    cat > "${STUB_BIN}/id" << 'EOF'
#!/bin/bash
echo "0"
EOF
    chmod +x "${STUB_BIN}/id"

    cat > "${MATRIXOS_DEV_DIR}/dev/vector_builder.sh" << 'EOF'
#!/bin/bash
echo "ARGS: $@"
EOF
    chmod +x "${MATRIXOS_DEV_DIR}/dev/vector_builder.sh"

    run main --foo --bar
    [ "$status" -eq 0 ]
    [[ "$output" == *"ARGS: --on-build-server --disable-send-mail --foo --bar"* ]]
}

@test "main skips root check and warning when --help is passed" {
    cat > "${MATRIXOS_DEV_DIR}/dev/vector_builder.sh" << 'EOF'
#!/bin/bash
echo "ARGS: $@"
EOF
    chmod +x "${MATRIXOS_DEV_DIR}/dev/vector_builder.sh"

    run main --help
    [ "$status" -eq 0 ]
    # Should NOT print the build warning
    [[ "$output" != *"ATTENTION PLEASE"* ]]
    [[ "$output" == *"ARGS: --on-build-server --disable-send-mail --help"* ]]
}
