#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法:
  ./scripts/fix-perms.sh

说明:
  处理“之前用 sudo 跑过 Vea 导致 data/、artifacts/、/tmp/vea-debug.log 变成 root 拥有”的情况。
  脚本会通过 pkexec 只弹一次授权，然后把 *root-owned* 的条目修回当前用户。
EOF
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_FILE="/tmp/vea-debug.log"
DATA_DIR="${ROOT_DIR}/data"
ARTIFACTS_DIR="${ROOT_DIR}/artifacts"

AS_ROOT=0
TARGET_UID=""
TARGET_GID=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --as-root)
      AS_ROOT=1
      shift
      ;;
    --uid)
      TARGET_UID="${2:-}"
      shift 2
      ;;
    --gid)
      TARGET_GID="${2:-}"
      shift 2
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$AS_ROOT" -eq 0 ]]; then
  if ! command -v pkexec >/dev/null 2>&1; then
    echo "未找到 pkexec（polkit）。请安装后重试。" >&2
    exit 2
  fi

  uid="$(id -u)"
  gid="$(id -g)"

  echo "[Vea] 将修复以下路径中 root-owned 的条目："
  echo "  - ${LOG_FILE}"
  echo "  - ${DATA_DIR}"
  echo "  - ${ARTIFACTS_DIR}"
  echo "[Vea] 将通过 pkexec 请求一次管理员授权..."

  exec pkexec "$0" --as-root --uid "${uid}" --gid "${gid}"
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "该脚本的 root 阶段必须由 pkexec 运行" >&2
  exit 2
fi
if [[ -z "${TARGET_UID}" || -z "${TARGET_GID}" ]]; then
  echo "缺少 --uid/--gid" >&2
  exit 2
fi
if [[ "${ROOT_DIR}" == "/" ]]; then
  echo "拒绝在 / 上执行" >&2
  exit 2
fi

fix_tree() {
  local path="$1"
  if [[ ! -e "$path" ]]; then
    return 0
  fi
  if [[ "$path" == "/" ]]; then
    echo "拒绝修复 /" >&2
    exit 2
  fi

  if [[ -d "$path" ]]; then
    find "$path" -xdev -user root -exec chown -h "${TARGET_UID}:${TARGET_GID}" {} + || true
    return 0
  fi

  if [[ -e "$path" ]]; then
    # 只处理 root-owned 文件，避免误伤（比如 TUN 的 vea-tun 所有者文件）
    if [[ "$(stat -c '%u' "$path")" == "0" ]]; then
      chown -h "${TARGET_UID}:${TARGET_GID}" "$path" || true
    fi
  fi
}

# /tmp 的调试日志：直接删掉，后续由用户态重新创建
rm -f "${LOG_FILE}" || true

fix_tree "${DATA_DIR}"
fix_tree "${ARTIFACTS_DIR}"

echo "[Vea] 权限修复完成"

