#!/bin/bash
# Vea Linux 管理员模式安装脚本
# 由 electron-builder 在安装后自动调用

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "[Vea] 安装管理员模式支持..."

# electron-builder 的 deb afterInstall 脚本会放到 dpkg 的 info 目录，不能依赖 SCRIPT_DIR。
# 这里优先使用脚本所在目录，其次尝试 deb 默认安装目录 /opt/Vea/resources/scripts。
resolve_resource_scripts_dir() {
  if [ -f "$SCRIPT_DIR/vea-admin" ]; then
    echo "$SCRIPT_DIR"
    return 0
  fi

  if [ -f "/opt/Vea/resources/scripts/vea-admin" ]; then
    echo "/opt/Vea/resources/scripts"
    return 0
  fi

  for dir in /opt/*/resources/scripts; do
    if [ -f "$dir/vea-admin" ]; then
      echo "$dir"
      return 0
    fi
  done

  return 1
}

RESOURCE_SCRIPTS_DIR=""
if RESOURCE_SCRIPTS_DIR="$(resolve_resource_scripts_dir)"; then
  :
else
  echo "[Vea] Error: 无法定位资源脚本目录（期望 /opt/Vea/resources/scripts）" >&2
  exit 2
fi

# 1. 安装 wrapper 脚本到 /usr/bin
if [ -f "$RESOURCE_SCRIPTS_DIR/vea-admin" ]; then
  install -Dm755 "$RESOURCE_SCRIPTS_DIR/vea-admin" /usr/bin/vea-admin
  echo "[Vea] ✓ 已安装 /usr/bin/vea-admin"
fi

# 2. 安装 polkit policy
if [ -f "$RESOURCE_SCRIPTS_DIR/com.veaproxy.vea.policy" ]; then
  install -Dm644 "$RESOURCE_SCRIPTS_DIR/com.veaproxy.vea.policy" \
    /usr/share/polkit-1/actions/com.veaproxy.vea.policy
  echo "[Vea] ✓ 已安装 polkit policy"
fi

# 3. 安装 desktop entry
if [ -f "$RESOURCE_SCRIPTS_DIR/vea-root.desktop" ]; then
  install -Dm644 "$RESOURCE_SCRIPTS_DIR/vea-root.desktop" \
    /usr/share/applications/vea-root.desktop
  echo "[Vea] ✓ 已安装桌面快捷方式"

  # 更新桌面数据库
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/share/applications 2>/dev/null || true
  fi
fi

echo "[Vea] 管理员模式安装完成"
echo "[Vea] 提示：在应用列表中可找到「Vea (管理员模式)」图标"
