# Steam Deck Seeder for MatrixOS

This seeder customizes MatrixOS for **Steam Deck (Jupiter APU)** compatibility.
It includes:

- **Custom Kernel (`sys-kernel/matrixos-kernel`)** with Steam Deck patches.
- **AMDGPU Firmware** for Van Gogh APU (e.g., `vega20_sdma.bin`).
- **Mesa with Vulkan/RADV** for hardware acceleration.
- **Udev rules** for Steam Deck input devices.

## Key Features
| Component | Purpose |
|-----------|---------|
| `sys-kernel/matrixos-kernel` | Kernel with `DRM_AMDGPU`, `DRM_AMD_DC`, and Jupiter patches. |
| `sys-firmware/amd-gpu-firmware` | AMD GPU firmware for Steam Deck. |
| `media-libs/mesa` | Vulkan (`+vulkan`), RADV (`+radeon`), Wayland (`+wayland`). |
| `app-laptop/steamdeck-controls` | Steam Deck input/LED control. |

## Requirements
- **x86_64-v3 CPU** (Steam Deck’s Zen 2 APU).
- **At least 8GB RAM** (for building).
- **32GB+ storage** (for the image).

## Build Instructions
1. **Switch to a mutable branch** (if using OSTree):
   ```bash
   sudo vector branch switch matrixos/amd64/dev/gnome-full
   sudo vector jailbreak
   ```
2. **Build the image**:
   ```bash
   sudo ./dev/build.sh --force-images
   ```
3. **Flash to Steam Deck**:
   ```bash
   xz -d out/images/matrixos_amd64_steamdeck-*.img.xz
   dd if=out/images/matrixos_amd64_steamdeck-*.img of=/dev/mmcblk0 bs=4M status=progress
   ```

## Debugging
### Common Issues
| Issue | Solution |
|-------|----------|
| **Infinite boot loop** | Ensure `amdgpu` kernel module is loaded (`lsmod | grep amdgpu`). |
| **Read-only filesystem** | Use `sudo vector readwrite` before `emerge`. |
| **Missing firmware** | Verify `/lib/firmware/amdgpu/` contains `vega20_*.bin`. |
| **GPU not detected** | Check `dmesg | grep -i amdgpu` for errors. |

### Verify GPU
```bash
glxinfo | grep -i "renderer"  # Should show "AMD Van Gogh"
vulkaninfo | grep -i "gpu"    # Should show RADV
```

## Notes
- **Jupiter Patches**: Applied via `sys-kernel/matrixos-kernel`.
- **Firmware**: Bundled in `files/amdgpu-firmware.tar.xz`.
- **Immutability**: Use `vector readwrite` for temporary writes.