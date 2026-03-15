#!/usr/bin/env bats
# Unit tests for dev/cdn_pusher.example.sh

setup() {
    TEST_TMPDIR="$(mktemp -d)"
    export MATRIXOS_DEV_DIR="${TEST_TMPDIR}/matrixos"

    LOCAL_IMAGES_DIR="${MATRIXOS_DEV_DIR}/out/images"
    INDEX_FILE="${LOCAL_IMAGES_DIR}/index.html"
    LATEST_FILE="${LOCAL_IMAGES_DIR}/LATEST"
    LOCAL_OSTREE_REPO="${MATRIXOS_DEV_DIR}/ostree/repo"

    mkdir -p "${LOCAL_IMAGES_DIR}"
    mkdir -p "${LOCAL_OSTREE_REPO}"

    STUB_BIN="${TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # Load functions without running main.
    local tmp="${TEST_TMPDIR}/_script.sh"
    sed '/^main "\${@}"$/d' "${BATS_TEST_DIRNAME}/cdn_pusher.example.sh" > "${tmp}"
    source "${tmp}"
}

teardown() {
    rm -rf "${TEST_TMPDIR}"
}

# --- prepare_latest_file ---

@test "prepare_latest_file creates LATEST with latest date files" {
    touch "${LOCAL_IMAGES_DIR}/matrixos-gnome-20250101.img.xz"
    touch "${LOCAL_IMAGES_DIR}/matrixos-server-20250101.img.xz"
    touch "${LOCAL_IMAGES_DIR}/matrixos-gnome-20250108.img.xz"

    prepare_latest_file

    [ -f "${LATEST_FILE}" ]
    grep -q "20250108" "${LATEST_FILE}"
    ! grep -q "20250101" "${LATEST_FILE}"
}

@test "prepare_latest_file creates empty LATEST when no img.xz" {
    prepare_latest_file

    [ -f "${LATEST_FILE}" ]
    [ ! -s "${LATEST_FILE}" ]
}

@test "prepare_latest_file picks up only the latest date" {
    touch "${LOCAL_IMAGES_DIR}/matrixos-server-20250201.img.xz"
    touch "${LOCAL_IMAGES_DIR}/matrixos-gnome-20250201.img.xz"
    touch "${LOCAL_IMAGES_DIR}/matrixos-cosmic-20250108.img.xz"

    prepare_latest_file

    grep -q "20250201" "${LATEST_FILE}"
    ! grep -q "20250108" "${LATEST_FILE}"
}

# --- prepare_index_html ---

@test "prepare_index_html creates index.html with file entries" {
    echo "data" > "${LOCAL_IMAGES_DIR}/somefile.img.xz"

    prepare_index_html

    [ -f "${INDEX_FILE}" ]
    grep -q "MatrixOS Images" "${INDEX_FILE}"
    grep -q "somefile.img.xz" "${INDEX_FILE}"
}

@test "prepare_index_html creates valid HTML structure" {
    echo "data" > "${LOCAL_IMAGES_DIR}/test.img.xz"

    prepare_index_html

    grep -q "<!DOCTYPE html>" "${INDEX_FILE}"
    grep -q "</html>" "${INDEX_FILE}"
    grep -q "<title>MatrixOS Images</title>" "${INDEX_FILE}"
}

@test "prepare_index_html excludes index.html from listing" {
    echo "data" > "${LOCAL_IMAGES_DIR}/test.img.xz"

    prepare_index_html

    # index.html itself should not appear in the file listing
    local count
    count=$(grep -c "index.html" "${INDEX_FILE}" || true)
    # Only the title and not in file links
    ! grep -q 'href="index.html"' "${INDEX_FILE}"
}

@test "prepare_index_html handles empty directory" {
    prepare_index_html

    [ -f "${INDEX_FILE}" ]
    grep -q "MatrixOS Images" "${INDEX_FILE}"
}

# --- main ---

@test "main warns when no releases and no images" {
    export MATRIXOS_BUILT_RELEASES=""
    export MATRIXOS_BUILT_IMAGES="0"

    run main
    [[ "$output" == *"No new releases were built"* ]]
    [[ "$output" == *"No new images were built"* ]]
}

@test "main calls push_cloudflare_ostree when releases exist" {
    push_cloudflare_ostree() { echo "OSTREE_PUSHED"; }
    export MATRIXOS_BUILT_RELEASES="release1 release2"
    export MATRIXOS_BUILT_IMAGES="0"

    run main
    [[ "$output" == *"OSTREE_PUSHED"* ]]
    [[ "$output" == *"No new images were built"* ]]
}

@test "main calls push_cloudflare_images when images built" {
    push_cloudflare_images() { echo "IMAGES_PUSHED"; }
    export MATRIXOS_BUILT_RELEASES=""
    export MATRIXOS_BUILT_IMAGES="1"

    run main
    [[ "$output" == *"IMAGES_PUSHED"* ]]
    [[ "$output" == *"No new releases were built"* ]]
}

@test "main calls all push functions when releases and images" {
    push_cloudflare_ostree() { echo "OSTREE_PUSHED"; }
    push_cloudflare_images() { echo "IMAGES_PUSHED"; }
    export MATRIXOS_BUILT_RELEASES="branch1"
    export MATRIXOS_BUILT_IMAGES="1"

    run main
    [[ "$output" == *"OSTREE_PUSHED"* ]]
    [[ "$output" == *"IMAGES_PUSHED"* ]]
}
