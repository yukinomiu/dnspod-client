# 腾讯云DNSPod DDNS客户端

## 用法

1. 下载对应平台二进制可执行文件
2. 运行

```
./dnspod-client -c /path/to/configfile.json
```

## 配置文件说明

- 配置文件格式为`JSON`
- 程序启动时, 如果未通过参数`-c`指定配置文件地址, 则默认加载当前目录下的`./config.json`文件
- 配置文件参数说明:

```
{
  "intervalS": 3600, // 更新时间间隔, 单位为秒, 比如3600(1小时)
  "secretKey": "secretKey", // 账户对应的SecretKey
  "secretId": "secretId", // 账户对应的SecretId
  "modifyAtStartup": true, // 是否在程序启动时, 立即更新DDNS记录; true: 是; false: 否
  "domain": "domain", // 要更新的域名, 比如 yourdomain.com
  "subDomain": "subDomain", // 要更新的子域名, 比如 home.yourdomain.com
  "recordId": 1234, // 账户对应的域名ID
  "recordLine": "recordLine", // 线路, 比如"默认", "联通"
  "ttl": 3600 // 域名TTL
}
```