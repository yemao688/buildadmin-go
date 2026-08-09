#!/usr/bin/env bash
# ============================================================
# cmd/bench/run-bench.sh — 后端压测一键复测
#
# 固化压测正确姿势（release 二进制 + app.env=release + 关 SQL 日志 +
# 独立端口），把"临时改 configs/config.yaml → 压测 → 还原"自动化：
#
#   1. 构建 release 二进制：go build -ldflags '-s -w' -o runtime/tmp/ba-server-release ./cmd/server
#   2. 备份 configs/config.yaml；以 cmd/bench/bench-config.yaml 为蓝本，把
#      <MYSQL_*> / <TOKEN_KEY> 占位符替换为 configs/config.yaml 的真实值
#      （mysql 连接段 + token.key 自动提取，YAML 单引号包裹防注入），生成
#      临时运行配置并替换 configs/config.yaml —— 服务从仓库根启动时固定分层
#      加载 configs/config.defaults.yaml + configs/config.yaml，-c/--conf 会把
#      defaults 基准目录也切走，不能用于指定独立压测配置
#   3. APP_PORT=$port 启动服务（独立端口，默认 9904，不干扰 9902 air）
#   4. 轮询 /healthz 就绪后跑压测：默认内置两个推荐场景 × {c=50, c=100}，
#      或 --raw 把剩余参数原样透传给 go run ./cmd/bench
#   5. trap EXIT/INT/TERM 兜底：任何中途失败、Ctrl+C 都还原 configs/config.yaml
#      并杀掉压测服务
#
# 用法：
#   ./cmd/bench/run-bench.sh                                   # 默认两场景 × {c=50, c=100}
#   ./cmd/bench/run-bench.sh -p 9905                           # 指定端口（默认 9904）
#   ./cmd/bench/run-bench.sh -n 1000                           # 每场景每并发请求数（默认 2000）
#   ./cmd/bench/run-bench.sh -t '<batoken>'                    # 追加后台 admin 场景（或设 BENCH_ADMIN_TOKEN）
#   ./cmd/bench/run-bench.sh --raw -url http://127.0.0.1:9904/api/index/index \
#       -n 500 -c 20 -method POST -body '{"x":1}'              # 透传任意 bench 参数
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMPLATE="$SCRIPT_DIR/bench-config.yaml"
CONF="$REPO_ROOT/configs/config.yaml"
SERVER_BIN="$REPO_ROOT/runtime/tmp/ba-server-release"
BENCH_LOG="/tmp/ba-bench.log"

PORT=9904
REQUESTS=2000
ADMIN_TOKEN="${BENCH_ADMIN_TOKEN:-}"
RAW_MODE=0
raw_args=()

SERVER_PID=""
BACKUP=""
AWK_HELPER=""
RENDER_AWK=""

cd "$REPO_ROOT"

usage() {
  cat <<'EOF'
用法: run-bench.sh [选项] [--raw <go run ./cmd/bench 参数...>]

  -p <port>   服务与压测端口（默认 9904；独立端口，不干扰 9902 air）
  -n <n>      每个场景每个并发级别的请求数（默认 2000）
  -t <token>  管理员 batoken，启用后台 admin 场景（也可用环境变量 BENCH_ADMIN_TOKEN）
  -h          显示本帮助
  --raw       透传模式：其后所有参数原样交给 go run ./cmd/bench（自行指定 -url 等）

默认模式（不传 --raw）内置两个推荐场景：
  1. GET /api/index/index（公开，无需 token）   × {c=50, c=100}
  2. GET /admin/auth.Admin/index（需 batoken）  × {c=50, c=100}（提供 -t 后运行）

示例：
  ./cmd/bench/run-bench.sh
  ./cmd/bench/run-bench.sh -p 9905 -n 1000 -t '<batoken>'
  ./cmd/bench/run-bench.sh --raw -url http://127.0.0.1:9904/api/index/index -n 500 -c 20
EOF
}

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
say() { printf '\n==> %s\n' "$*"; }

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -p) PORT="${2:?usage: -p 需要端口参数}"; shift 2 ;;
      -n) REQUESTS="${2:?usage: -n 需要请求数参数}"; shift 2 ;;
      -t) ADMIN_TOKEN="$2"; shift 2 ;;
      -h|--help) usage; exit 0 ;;
      --raw) RAW_MODE=1; shift; raw_args=("$@"); break ;;
      *) die "未知参数: $1（见 -h）" ;;
    esac
  done
  [[ "$PORT" =~ ^[0-9]+$ ]] || die "端口必须是数字: $PORT"
  [[ "$REQUESTS" =~ ^[0-9]+$ && "$REQUESTS" -gt 0 ]] || die "-n 必须是正整数: $REQUESTS"
  if [[ "$RAW_MODE" -eq 1 && ${#raw_args[@]} -eq 0 ]]; then
    die "--raw 后需要提供 bench 参数（如 -url http://127.0.0.1:9904/api/index/index）"
  fi
}

# 两个 awk 辅助程序（heredoc 写入临时文件，避免引号转义地狱）
setup_awk() {
  AWK_HELPER="$(mktemp)"
  RENDER_AWK="$(mktemp)"
  cat > "$AWK_HELPER" <<'AWKEOF'
# 提取扁平 YAML 覆盖层（安装器 WriteConfigOverrides 生成格式）中 <sec>.<key> 的标量值：
# 段感知（只匹配顶层段下的 key）、引号感知（单引号 '' 转义 / 双引号 \" 转义）、
# 注释感知（未加引号的值剥离行内注释，即引号外的 #）。
BEGIN { cur = "" }
/^[^[:space:]#]/ { cur = $1; sub(/:$/, "", cur) }
cur == sec && $1 == key ":" {
    line = substr($0, index($0, ":") + 1)
    sub(/^[[:space:]]+/, "", line)
    if (line == "") { print ""; exit }
    if (substr(line, 1, 1) == "\"") {
        out = ""; rest = substr(line, 2)
        while (rest != "") {
            c = substr(rest, 1, 1)
            if (c == "\\") { out = out substr(rest, 2, 1); rest = substr(rest, 3); continue }
            if (c == "\"") { print out; exit }
            out = out c; rest = substr(rest, 2)
        }
        print out; exit
    }
    if (substr(line, 1, 1) == "'") {
        out = ""; rest = substr(line, 2)
        while (rest != "") {
            c = substr(rest, 1, 1)
            if (c == "'") {
                if (substr(rest, 2, 1) == "'") { out = out "'"; rest = substr(rest, 3); continue }
                print out; exit
            }
            out = out c; rest = substr(rest, 2)
        }
        print out; exit
    }
    sub(/[[:space:]]+#.*$/, "", line)
    sub(/[[:space:]]+$/, "", line)
    print line
    exit
}
AWKEOF
  cat > "$RENDER_AWK" <<'AWKEOF'
# 把模板占位符替换为真实值：index() 字面匹配（值可含任意字符），
# 全部以 YAML 单引号包裹（内部 ' 转义为 ''）防注入。
# 模板中占位符已带引号（如 host: '<MYSQL_HOST>'），整体替换避免双重引号。
function q(v, out) {
    out = v
    gsub(/'/, "''", out)
    return "'" out "'"
}
function subline(line, tok, val, i) {
    while ((i = index(line, tok)) > 0)
        line = substr(line, 1, i - 1) q(val) substr(line, i + length(tok))
    return line
}
{
    line = $0
    if (host != "") line = subline(line, "'<MYSQL_HOST>'", host)
    if (port != "") line = subline(line, "<MYSQL_PORT>", port)
    if (db != "") line = subline(line, "'<MYSQL_DATABASE>'", db)
    if (pfx != "") line = subline(line, "'<MYSQL_PREFIX>'", pfx)
    if (user != "") line = subline(line, "'<MYSQL_USERNAME>'", user)
    if (pass != "") line = subline(line, "'<MYSQL_PASSWORD>'", pass)
    if (tkey != "") line = subline(line, "'<TOKEN_KEY>'", tkey)
    # 非注释行仍含占位符 = 对应值在 configs/config.yaml 缺失，
    # 省略该行让 config.defaults.yaml 的默认值生效（password 缺省值本就是空）
    if (line !~ /^[[:space:]]*#/ && line ~ /<MYSQL_|<TOKEN_KEY>/) next
    print line
}
AWKEOF
}

# yaml_scalar <section> <key>：从 configs/config.yaml 提取标量值
yaml_scalar() { awk -v sec="$1" -v key="$2" -f "$AWK_HELPER" "$CONF"; }

extract_config() {
  MYSQL_HOST=$(yaml_scalar mysql host)
  MYSQL_PORT=$(yaml_scalar mysql port)
  MYSQL_DATABASE=$(yaml_scalar mysql database)
  MYSQL_PREFIX=$(yaml_scalar mysql prefix)
  MYSQL_USERNAME=$(yaml_scalar mysql username)
  MYSQL_PASSWORD=$(yaml_scalar mysql password)
  TOKEN_KEY=$(yaml_scalar token key)

  [[ -n "$TOKEN_KEY" ]] || die "configs/config.yaml 未找到 token.key——请先运行 setup 完成安装（它写入随机 token.key）"
  [[ -n "$MYSQL_DATABASE" && -n "$MYSQL_USERNAME" ]] || die "configs/config.yaml 的 mysql 段不完整（缺 database/username）——请先运行 setup 完成安装"
}

build_server() {
  say "构建 release 二进制: ${SERVER_BIN#$REPO_ROOT/}"
  mkdir -p "$(dirname "$SERVER_BIN")"
  go build -ldflags '-s -w' -o "$SERVER_BIN" ./cmd/server
}

prepare_config() {
  say "生成临时压测配置（蓝本 ${TEMPLATE#$REPO_ROOT/} + configs/config.yaml 的 mysql/token 值）"
  local tmp_conf
  tmp_conf="$(mktemp "${CONF}.bench-run.XXXXXX")"
  BACKUP="$(mktemp "${CONF}.bench-bak.XXXXXX")"

  render_config > "$tmp_conf"

  # 模板中因 configs/config.yaml 缺失对应值而被省略的行（回退默认值）；
  # 只检查非注释行（模板头注释里含占位符说明文字）
  local dropped
  dropped="$(grep -v '^[[:space:]]*#' "$tmp_conf" | grep -o '<MYSQL_[A-Z_]*>' | sort -u || true)"
  if [[ -n "$dropped" ]]; then
    echo "warning: 以下字段在 configs/config.yaml 中缺失，对应配置行已省略（回退 config.defaults.yaml 默认值）：" >&2
    echo "$dropped" | sed 's/^/    /' >&2
  fi

  say "备份并临时替换 configs/config.yaml"
  cp "$CONF" "$BACKUP"
  cp "$tmp_conf" "$CONF"
  rm -f "$tmp_conf"
}

render_config() {
  awk -v host="$MYSQL_HOST" -v port="$MYSQL_PORT" -v db="$MYSQL_DATABASE" \
      -v pfx="$MYSQL_PREFIX" -v user="$MYSQL_USERNAME" -v pass="$MYSQL_PASSWORD" \
      -v tkey="$TOKEN_KEY" -f "$RENDER_AWK" "$TEMPLATE"
}

start_server() {
  if curl -fsS --max-time 2 "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1; then
    die "端口 ${PORT} 已有服务在响应 /healthz（可能是残留的压测服务），请先停掉它或换 -p <port>"
  fi
  say "启动压测服务: APP_PORT=${PORT} ${SERVER_BIN#$REPO_ROOT/}（日志: ${BENCH_LOG}）"
  APP_PORT="$PORT" "$SERVER_BIN" > "$BENCH_LOG" 2>&1 &
  SERVER_PID=$!
  echo "pid=$SERVER_PID"
}

wait_healthz() {
  say "等待 /healthz 就绪 ..."
  local url="http://127.0.0.1:${PORT}/healthz" i
  for i in $(seq 1 60); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      echo "ready: $url"
      return 0
    fi
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
      echo "压测服务启动失败（进程已退出），日志末尾：" >&2
      tail -n 30 "$BENCH_LOG" >&2 || true
      return 1
    fi
    sleep 0.5
  done
  echo "30s 内 /healthz 未就绪，日志末尾：" >&2
  tail -n 30 "$BENCH_LOG" >&2 || true
  return 1
}

run_scenario() {
  local url="$1" c="$2" hdr="${3:-}"
  echo
  echo "---- ${url}（c=${c}, n=${REQUESTS}）----"
  if [[ -n "$hdr" ]]; then
    go run ./cmd/bench -url "$url" -n "$REQUESTS" -c "$c" -header "$hdr"
  else
    go run ./cmd/bench -url "$url" -n "$REQUESTS" -c "$c"
  fi
}

run_bench() {
  local base="http://127.0.0.1:${PORT}"
  if [[ "$RAW_MODE" -eq 1 ]]; then
    say "透传模式: go run ./cmd/bench ${raw_args[*]}"
    go run ./cmd/bench "${raw_args[@]}"
    return
  fi

  say "默认场景 1/2：前台初始化接口（公开，免 token）GET ${base}/api/index/index × {c=50, c=100}"
  run_scenario "$base/api/index/index" 50
  run_scenario "$base/api/index/index" 100

  if [[ -n "$ADMIN_TOKEN" ]]; then
    say "默认场景 2/2：后台列表接口（batoken）GET ${base}/admin/auth.Admin/index × {c=50, c=100}"
    run_scenario "$base/admin/auth.Admin/index" 50 "batoken: ${ADMIN_TOKEN}, think-lang: zh-cn"
    run_scenario "$base/admin/auth.Admin/index" 100 "batoken: ${ADMIN_TOKEN}, think-lang: zh-cn"
  else
    echo
    echo "（跳过后台 admin 场景：未提供管理员 token；可用 -t <batoken> 或环境变量 BENCH_ADMIN_TOKEN 启用）"
  fi
}

cleanup() {
  local rc=$?
  # 1) 停止压测服务（TERM 优雅关闭；5s 内未退出再 KILL）
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    echo
    echo "==> 停止压测服务 (pid $SERVER_PID) ..."
    kill "$SERVER_PID" 2>/dev/null || true
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$SERVER_PID" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$SERVER_PID" 2>/dev/null; then
      kill -9 "$SERVER_PID" 2>/dev/null || true
    fi
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  # 2) 还原 configs/config.yaml
  if [[ -n "$BACKUP" && -f "$BACKUP" ]]; then
    cp "$BACKUP" "$CONF"
    rm -f "$BACKUP"
    echo "==> configs/config.yaml 已还原"
  fi
  [[ -n "$AWK_HELPER" ]] && rm -f "$AWK_HELPER"
  [[ -n "$RENDER_AWK" ]] && rm -f "$RENDER_AWK"
  exit "$rc"
}
trap cleanup EXIT INT TERM

main() {
  parse_args "$@"
  [[ -f "$CONF" ]] || die "缺少 ${CONF}——请先运行 setup 完成安装（它写入 mysql 连接与随机 token.key）"
  [[ -f "$TEMPLATE" ]] || die "缺少模板 $TEMPLATE"
  setup_awk
  build_server
  extract_config
  prepare_config
  start_server
  wait_healthz
  run_bench
  echo
  echo "==> 压测完成。configs/config.yaml 已还原，压测服务已停止。基线请记录到 cmd/bench/README.md"
}

main "$@"
