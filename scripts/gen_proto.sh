#!/bin/bash
set -e

if [ $# -ne 2 ]; then
    echo "错误，请传入输出目录和proto文件的绝对路径"
    exit 1
fi

# 等号前后不能有空格
OUTPUT_DIR="${1}"
PROTO_FILE_PATH="${2}"

mkdir -p "${OUTPUT_DIR}"


echo "输出目录: ${OUTPUT_DIR}"
echo "Proto文件: ${PROTO_FILE_PATH}"


if [ ! -f "${PROTO_FILE_PATH}" ]; then
    echo "错误，传入的proto文件路径不存在: ${PROTO_FILE_PATH}"
    exit 1
fi

# 校验proto相关工具是否安装
command -v protoc >/dev/null 2>&1 || { echo "错误：未安装 protoc，请先安装 Protocol Buffers 编译器"; exit 1; }
command -v protoc-gen-go >/dev/null 2>&1 || { echo "错误：未安装 protoc-gen-go，请执行 go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
command -v protoc-gen-go-grpc >/dev/null 2>&1 || { echo "错误：未安装 protoc-gen-go-grpc，请执行 go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }

PROTO_DIR=$(dirname "${PROTO_FILE_PATH}")

GOPATH=$(go env GOPATH)

protoc -I ./ \
	-I $GOPATH/pkg \
	-I $GOPATH/pkg/mod/github.com/googleapis/googleapis@v0.0.0-20260123134045-2ac88973cbaf \
    --proto_path="${PROTO_DIR}" \
    --go_out="${OUTPUT_DIR}" --go_opt=paths=source_relative \
    --go-grpc_out="${OUTPUT_DIR}" --go-grpc_opt=paths=source_relative \
    "${PROTO_FILE_PATH}"

echo "${PROTO_FILE_NAME} 处理完成！"
echo "生成的文件在: ${OUTPUT_DIR}"