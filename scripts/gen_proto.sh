#!/bin/bash
set -e

if [ $# -eq 0]; then
    echo "错误，请传入proto文件路径"
    exit 1
fi

# 等号前后不能有空格
PROTO_FILE_PATH="${1}"

if [ ! -f "${PROTO_FILE_PATH}" ]; then
    echo "错误，传入的proto文件路径不存在: ${PROTO_FILE_PATH}"
    exit 1
fi

# 校验proto相关工具是否安装
command -v protoc >/dev/null 2>&1 || { echo "错误：未安装 protoc，请先安装 Protocol Buffers 编译器"; exit 1; }
command -v protoc-gen-go >/dev/null 2>&1 || { echo "错误：未安装 protoc-gen-go，请执行 go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "错误：未安装 protoc-gen-go-grpc，请执行 go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }


# 生成不同模块的proto代码
protoc --proto_path=. \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    "${PROTO_FILE_PATH}"

echo "✅ 基于 ${PROTO_FILE_PATH} 生成的proto代码完成"