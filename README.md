# 概述
从 ceph-mgr 的 8443 端口获取 对象存储 的指标数据

# 构建
```shell
docker build -f Dockerfile -t lchdzh/e37-exporter:$(cat VERSION) .
```

# 运行
## exporter 请求控制台代理
部署在湖外
```shell
docker run -d --name=e37-exporter --net=host \
lchdzh/e37-exporter:$(cat VERSION) \
--web.listen-address=':18443' \
--e37-server='http://ss-admin.datalake.gdmm.ehualu.it:38443'
```


## 测试
```shell
go run cmd/e37_exporter/main.go --web.listen-address=':18443' --log-level=debug --e37-server='http://ss-admin.datalake.gdmm.ehualu.it:38443'
```

```shell
docker run --rm --name=e37-exporter --net=host lchdzh/e37-exporter:$(cat VERSION) --web.listen-address=':18443' --log-level=debug --e37-server='http://ss-admin.datalake.gdmm.ehualu.it:38443'
```
