#!/bin/bash

# 检查是否已经有服务在运行
if [[ -n $(pgrep -f "main") ]]; then
  # 发送 SIGTERM 信号通知旧进程优雅关闭
  pkill -f "main"s
fi

# 启动新进程
nohup ./main &

# 输出新进程的 PID
echo $!