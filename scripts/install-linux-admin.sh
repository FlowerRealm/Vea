#!/bin/bash
# Vea Linux 管理员模式安装脚本
# 由 electron-builder 在安装后自动调用

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[Vea] 安装管理员模式支持..."

# 1. 安装 wrapper 脚本到 /usr/bin
if [ -f "$SCRIPT_DIR/vea-admin" ]; then
    install -Dm755 "$SCRIPT_DIR/vea-admin" /usr/bin/vea-admin
    echo "[Vea] ✓ 已安装 /usr/bin/vea-admin"
fi

# 2. 安装 polkit policy
if [ -f "$SCRIPT_DIR/com.veaproxy.vea.policy" ]; then
    install -Dm644 "$SCRIPT_DIR/com.veaproxy.vea.policy" \
        /usr/share/polkit-1/actions/com.veaproxy.vea.policy
    echo "[Vea] ✓ 已安装 polkit policy"
fi

# 3. 安装 desktop entry
if [ -f "$SCRIPT_DIR/vea-root.desktop" ]; then
    install -Dm644 "$SCRIPT_DIR/vea-root.desktop" \
        /usr/share/applications/vea-root.desktop
    echo "[Vea] ✓ 已安装桌面快捷方式"

    # 更新桌面数据库
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database /usr/share/applications 2>/dev/null || true
    fi
fi

echo "[Vea] 管理员模式安装完成"
echo "[Vea] 提示：在应用列表中可找到「Vea (管理员模式)」图标"
