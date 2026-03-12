1. `go get` timeout，即连接官方 Go 模块代理 proxy.golang.org 的 IPv6 地址超时：

    替换proxy ```go env -w GOPROXY=https://goproxy.cn,direct```

2. proto文件规范，aka. protobuf接口定义规范

3. 在运行protoc指令时，遇到问题```Import "google/api/field_behavior.proto" was not found or had errors.``` 。解决方法可以参考[这个帖子][https://www.cnblogs.com/yisany/p/14875488.html]。具体步骤：
   - 下载包：go get github.com/googleapis/googleapis
   - 查看下载到哪里了：go list -m -f '{{.Path}} => {{.Dir}}' github.com/googleapis/googleapis
   - 修改proto文件里的protoc command，加上-I，与包寻址有关

4. zap和viper不必细究，使用方案参照文档和一些教程就ok