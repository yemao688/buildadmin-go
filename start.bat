@echo off
setlocal enabledelayedexpansion

:: 检查是否已经有服务在运行
for /f "tokens=5" %%a in ('tasklist ^| find "main.exe"') do (
  :: 发送 CTRL+C 信号通知旧进程优雅关闭
  taskkill /F /PID %%a /T
)

:: 启动新进程
start "" main.exe

:: 输出新进程的 PID
endlocal