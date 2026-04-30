#!/bin/bash

# TMD Server 启动脚本
# 该脚本通过监听 tmd 进程的退出码来实现：
# 1. 正常关闭 (Exit 0) -> 退出脚本
# 2. 异常崩溃 (非 0) -> 等待 5 秒后自动拉起

# 确保执行的是编译后的二进制文件
BIN_PATH="./tmd"

# 检查二进制文件是否存在
if [ ! -f "$BIN_PATH" ]; then
    echo "Error: Executable not found at $BIN_PATH"
    echo "Please build the project first using 'go build -o tmd ./main.go'"
    exit 1
fi

echo "Starting TMD Server..."

while true; do
    # 启动服务器（使用你需要的参数，比如 -server 模式和 -port 指定端口）
    $BIN_PATH -server "$@"
    
    # 捕获退出码
    EXIT_CODE=$?
    
    if [ $EXIT_CODE -eq 0 ]; then
        echo "Server shut down gracefully (Exit Code 0). Stopping script."
        break
    else
        echo "Server crashed or stopped unexpectedly (Exit Code $EXIT_CODE). Starting again in 5 seconds..."
        sleep 5
    fi
done
