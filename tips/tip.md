1. `go get` timeout，即连接官方 Go 模块代理 proxy.golang.org 的 IPv6 地址超时：

    替换proxy ```go env -w GOPROXY=https://goproxy.cn,direct```