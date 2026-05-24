#!/bin/bash
set -e

# Ensure the AMDGPU firmware is prioritized
echo 'VIDEO_CARDS="amdgpu radeonsi"' >> /etc/portage/make.conf

# Rebuild mesa with the new Vulkan flags
emerge --update --newuse media-libs/mesa

# Set up the specialized Steam Deck input rules
if [ -d "/lib/udev/rules.d" ]; then
    echo 'KERNEL=="uinput", MODE="0660", GROUP="input", OPTIONS+="static_node=uinput"' > /lib/udev/rules.d/99-steamdeck-input.rules
fi
