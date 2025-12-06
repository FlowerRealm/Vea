#!/bin/bash
# Vea Linux 管理员模式卸载脚本

echo "[Vea] 卸载管理员模式支持..."

# 删除 wrapper
rm -f /usr/bin/vea-admin
echo "[Vea] ✓ 已删除 /usr/bin/vea-admin"

# 删除 polkit policy
rm -f /usr/share/polkit-1/actions/com.veaproxy.vea.policy
echo "[Vea] ✓ 已删除 polkit policy"

# 删除 desktop entry
rm -f /usr/share/applications/vea-root.desktop
echo "[Vea] ✓ 已删除桌面快捷方式"

# 更新桌面数据库
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/share/applications 2>/dev/null || true
fi

echo "[Vea] 管理员模式卸载完成"
