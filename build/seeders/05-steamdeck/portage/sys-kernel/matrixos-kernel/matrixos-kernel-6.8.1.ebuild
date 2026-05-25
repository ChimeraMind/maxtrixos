# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
LINUX_PATCHSET_DIR="${WORKDIR}/patches"
K_SECURITY_UNSUPPORTED=1
ETYPE="sources"
inherit linux-kernel-2

DESCRIPTION="MatrixOS Kernel with Steam Deck (Jupiter) patches"
HOMEPAGE="https://github.com/ChimeraMind/MaxtrixOS"
SRC_URI="https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.8.1.tar.xz
    https://github.com/ValveSoftware/SteamOS/archive/refs/tags/steamos-3.5.tar.gz -> steamos-3.5.tar.gz"
S="${WORKDIR}/linux-6.8.1"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="~amd64"

CONFIG_CHECK="~DRM_AMDGPU ~DRM_AMD_DC ~HSA_AMD"
PATCHES=(
    "${DISTDIR}/steamos-3.5/kernel/patches/0001-jupiter-6.8.1.patch"
    "${FILESDIR}/0002-enable-amdgpu-dc.patch"
)

pkg_postinst() {
    einfo "MatrixOS Kernel with Steam Deck support installed."
    einfo "Ensure /lib/firmware/amdgpu/ contains Steam Deck firmware."
}