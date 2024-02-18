## kubernetes mutatingwebhook demo

### 实现的特性
针对 deployment 资源
1. replicas 副本数最大为 3，如果超过 3 副本，则强制修改为 3。
2. 所有新增或更新的 deployment 资源，添加 env-type=true的 annotation。

### 部署方式
```shell
k apply -f deploy/webhook-server
k apply -f deploy/webhookconfiguration
```

### 启动命令
```shell
./mutating-demo -certFile /path/to/secret/tls.crt -keyFile /path/to/secret/tls.key
```
