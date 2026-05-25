#!/bin/bash
set -euo pipefail

# Make filesystem writable (for OSTree)
if ! vector readwrite; then
    echo "ERROR: Failed to make filesystem writable. Are you in an OSTree deployment?"
    echo "Try: sudo vector branch switch matrixos/amd64/dev/gnome-full && sudo vector jailbreak"
    exit 1
fi

# Ensure AMDGPU firmware is prioritized
echo 'VIDEO_CARDS="amdgpu radeonsi"' >> /etc/portage/make.conf || {
    echo "ERROR: Failed to update make.conf"
    exit 1
}

# Rebuild mesa with Vulkan flags
if ! emerge --update --newuse media-libs/mesa; then
    echo "ERROR: Failed to rebuild Mesa. Check logs in /var/log/portage/"
    exit 1
fi

# Verify amdgpu module
if ! modprobe amdgpu; then
    echo "ERROR: Failed to load amdgpu kernel module. Is the kernel built with DRM_AMDGPU?"
    echo "Check: zcat /proc/config.gz | grep -i DRM_AMDGPU"
    exit 1
fi

# Set up Steam Deck input rules
if [ -d "/lib/udev/rules.d" ]; then
    cat > /lib/udev/rules.d/99-steamdeck-input.rules <<'EOF'
KERNEL=="uinput", MODE="0660", GROUP="input", OPTIONS+="static_node=uinput"
EOF
fi

echo "Steam Deck seeder setup complete."