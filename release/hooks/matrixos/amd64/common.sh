#!/bin/bash
set -eu

release_common.chroot() {
    local chroot_dir="${1}"
    if [ -z "${chroot_dir}" ]; then
        echo "release_common.chroot: missing chroot_dir parameter" >&2
        return 1
    fi
    shift

    local chroot_exec="${1}"
    if [ -z "${chroot_exec}" ]; then
        echo "release_common.chroot: missing chroot_exec parameter" >&2
        return 1
    fi
    shift

    unshare \
        --pid \
        --fork \
        --kill-child \
        --mount \
        --uts \
        --ipc \
        --mount-proc="${chroot_dir}/proc" \
        chroot "${chroot_dir}" "${chroot_exec}" "${@}"
}


release_common.check_kernel_and_external_module() {
    local imagedir="${1}"
    local module_name="${2}"
    if [ -z "${module_name}" ] || [ -z "${imagedir}" ]; then
        echo "release_common.check_kernel_and_external_module: missing parameters imagedir and module name" >&2
        return 1
    fi

    local modulesdir="${imagedir}/lib/modules"

    local vmlinuzes=
    vmlinuzes=$(ls -1 "${imagedir}/lib/modules"/*/vmlinuz)
    local vmlinuz_count=0
    for vmlinuz in ${vmlinuzes}; do
        echo "Found kernel: ${vmlinuz}"
        vmlinuz_count=$((vmlinuz_count+1))
    done

    local initramfses=
    initramfses=$(ls -1 "${imagedir}/lib/modules"/*/initramfs)
    local initramfs_count=0
    for initramfs in ${initramfses}; do
        echo "Found initramfs: ${initramfs}"
        initramfs_count=$((initramfs_count+1))
    done

    if [ "${vmlinuz_count}" = "0" ]; then
        echo "No kernel found. Refusing to release." >&2
        return 1
    fi
    if [ "${initramfs_count}" = "0" ]; then
        echo "No initramfs found. Refusing to release." >&2
        return 1
    fi
    if [ "${vmlinuz_count}" != "${initramfs_count}" ]; then
        echo "vmlinuz found: ${vmlinuz_count} -- initramfs found: ${initramfs_count}. Refusing to release." >&2
        return 1
    fi

    local kernel_mods=
    mapfile -t kernel_mods < <(find "${modulesdir}" -type f -name "${module_name}")
    if [[ "${#kernel_mods[@]}" -eq 0 ]]; then
        echo "No ${module_name} found in ${modulesdir}" >&2
        return 1
    fi

    local kernel_mod=
    local kernel_mod_vermagic=
    local module_kernel_ver=
    local corresponding_vmlinuz=
    local vmlinuz_kernel_ver=
    local mod_count=0
    local failure=
    for kernel_mod in "${kernel_mods[@]}"; do
        kernel_mod="${kernel_mod#${imagedir%/}}"
        mod_count=$((mod_count+1))
        echo "Testing module: ${kernel_mod}"

        kernel_mod_vermagic=$(release_common.chroot "${imagedir}" modinfo -F vermagic "${kernel_mod}")
        module_kernel_ver=$(echo "${kernel_mod_vermagic}" | awk '{print $1}')
        echo "${kernel_mod}: vermagic is: ${kernel_mod_vermagic}, kernel ver is: ${module_kernel_ver}"

        corresponding_vmlinuz="${modulesdir}/${module_kernel_ver}/vmlinuz"
        if [ ! -e "${corresponding_vmlinuz}" ]; then
            echo "${corresponding_vmlinuz} not found for related ${kernel_mod}. Refusing to release." >&2
            failure=1
            continue
        fi

        vmlinuz_kernel_ver=$(file -b "${corresponding_vmlinuz}" | grep -oP 'version \K[^ ]+')
        if [ "${vmlinuz_kernel_ver}" != "${module_kernel_ver}" ]; then
            echo "${kernel_mod}: mismatch in kernel ver: (M) ${module_kernel_ver} vs (K) ${vmlinuz_kernel_ver}" >&2
            failure=1
            continue
        fi
    done
    if [ -n "${failure}" ]; then
        return 1
    fi

    if [ "${mod_count}" != "${vmlinuz_count}" ]; then
        echo "Unexpected number of ${module_name} files found! Refusing to release." >&2
        echo "Number of ${module_name} modules: ${mod_count} -- vmlinuz found: ${vmlinuz_count}" >&2
        echo "${kernel_mods[@]}" >&2
        return 1
    fi
}

release_common.check_nvidia_module() {
    local imagedir="${1}"
    if [ -z "${imagedir}" ]; then
        echo "release_common.check_nvidia_module: missing parameter imagedir" >&2
        return 1
    fi
    if [ ! -d "${imagedir}" ]; then
        echo "release_common.check_nvidia_module: ${imagedir} is not a directory" >&2
        return 1
    fi

    if [ ! -d "${imagedir}"/var/db/pkg/x11-drivers/nvidia-drivers* ]; then
        echo "x11-drivers/nvidia-drivers* not installed, skipping QA check"
        return 0
    fi
    release_common.check_kernel_and_external_module "${imagedir}" "nvidia.ko*"
}

release_common.check_ryzen_smu_module() {
    local imagedir="${1}"
    if [ -z "${imagedir}" ]; then
        echo "release_common.check_ryzen_smu_module: missing parameter imagedir" >&2
        return 1
    fi
    if [ ! -d "${imagedir}" ]; then
        echo "release_common.check_ryzen_smu_module: ${imagedir} is not a directory" >&2
        return 1
    fi

    if [ ! -d "${imagedir}"/var/db/pkg/app-admin/ryzen_smu* ]; then
        echo "app-admin/ryzen_smu* not installed, skipping QA check"
        return 0
    fi

    release_common.check_kernel_and_external_module "${imagedir}" "ryzen_smu.ko*"
}

release_common.list_top_packages() {
    local imagedir="${1}"
    if [ -z "${imagedir}" ]; then
        echo "release_common.list_top_packages: missing parameter imagedir" >&2
        return 1
    fi
    if [ ! -d "${imagedir}" ]; then
        echo "release_common.list_top_packages: ${imagedir} is not a directory" >&2
        return 1
    fi

    echo "Listing the top 20 largest packages:"
    release_common.chroot "${imagedir}" \
        equery size '*' | sed 's/\(.*\):.*(\(.*\))$/\2 \1/' \
            | sort -n | numfmt --to=iec-i | tail -n 20
}

release_common.check_leaking_buckets() {
    local imagedir="${1}"
    if [ -z "${imagedir}" ]; then
        echo "release_common.check_leaking_buckets: missing parameter imagedir" >&2
        return 1
    fi
    if [ ! -d "${imagedir}" ]; then
        echo "release_common.check_leaking_buckets: ${imagedir} is not a directory" >&2
        return 1
    fi

    local failure=

    local private_git_repo_path="${imagedir}/${MATRIXOS_DEFAULT_PRIVATE_GIT_REPO_PATH}"
    if [ -d "${private_git_repo_path}" ] && [ "$(ls -A "${private_git_repo_path}")" ]; then
        echo "ERROR: Leaking files found in private git repo path: ${MATRIXOS_DEFAULT_PRIVATE_GIT_REPO_PATH}" >&2
        echo "Leaking files:" >&2
        ls -la "${private_git_repo_path}" >&2
        failure=1
    fi

    # Scan for PEM-encoded private keys (SSH, GPG, X.509/secureboot, EC, etc.)
    # -I skips binary files (ELF, .so, firmware) that embed PEM strings in code.
    local leaking_key_files=()
    mapfile -t leaking_key_files < <(
        grep -rlFI \
            --exclude='*.xml' \
            --exclude='*.go' \
            --exclude='*.pem' \
            --exclude='*.html' \
            --exclude-dir='firmware' \
            --exclude-dir='doc' \
            -e '-----BEGIN OPENSSH PRIVATE KEY-----' \
            -e '-----BEGIN RSA PRIVATE KEY-----' \
            -e '-----BEGIN DSA PRIVATE KEY-----' \
            -e '-----BEGIN EC PRIVATE KEY-----' \
            -e '-----BEGIN PRIVATE KEY-----' \
            -e '-----BEGIN ENCRYPTED PRIVATE KEY-----' \
            -e '-----BEGIN PGP PRIVATE KEY BLOCK-----' \
            "${imagedir}" 2>/dev/null || true
    )
    if [ "${#leaking_key_files[@]}" -gt 0 ]; then
        echo "ERROR: Files containing private key material found:" >&2
        printf '  %s\n' "${leaking_key_files[@]}" >&2
        failure=1
    fi

    # Check for GnuPG private key storage (binary keybox format, not PEM)
    local gpg_private_dirs=()
    mapfile -t gpg_private_dirs < <(
        find "${imagedir}" -type d -name 'private-keys-v1.d' 2>/dev/null || true
    )
    local gpg_dir=
    for gpg_dir in "${gpg_private_dirs[@]}"; do
        if [ -n "$(ls -A "${gpg_dir}" 2>/dev/null)" ]; then
            echo "ERROR: GnuPG private keys found in: ${gpg_dir}" >&2
            ls -la "${gpg_dir}" >&2
            failure=1
        fi
    done

    # Check for well-known SSH private key filenames
    local ssh_key_files=()
    mapfile -t ssh_key_files < <(
        find "${imagedir}" -type f \
            \( -name 'id_rsa' -o -name 'id_ecdsa' -o -name 'id_ecdsa_sk' \
               -o -name 'id_ed25519' -o -name 'id_ed25519_sk' -o -name 'id_dsa' \
               -o -name 'ssh_host_*_key' \) \
            2>/dev/null || true
    )
    if [ "${#ssh_key_files[@]}" -gt 0 ]; then
        echo "ERROR: SSH private key files found:" >&2
        printf '  %s\n' "${ssh_key_files[@]}" >&2
        failure=1
    fi

    if [ -n "${failure}" ]; then
        return 1
    fi
}
