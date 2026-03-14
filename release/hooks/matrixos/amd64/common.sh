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