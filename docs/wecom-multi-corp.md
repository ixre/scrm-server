# 企微多企业接入

企微凭据由 `CrmChannelConfig` 管理。一个 corp 只能属于一个租户；不同企业可以使用相同的员工、客户、标签、群和消息外部 ID。

## 请求与同步

- 本地账号调用企微接口时，核心服务从当前租户解析启用的企业；没有配置时返回错误。
- 同一租户启用了多个企业时，请求通过 `X-Wecom-Corp-ID` 选择企业。核心服务检查归属，拒绝其他租户的 corp。企微员工会话固定使用登录时的企业。
- 登录链接必须传 `ext_corp_id`，登录回调校验 state 与企业；JS-SDK 使用对应企业及应用 ID。
- 后台同步和定时任务遍历启用的企业，各企业失败独立记录；客户同步队列携带 corp，不再使用全局凭据补全。
- 启动不会把全局 `WeWork` 配置自动分配给租户。旧数据导入应先建立明确的企业与租户映射。

## 自建应用

在「渠道设置」新增企微配置，填写该企业的应用凭据。回调地址为 `/wecom/callback/<corp_id>`，公网 HTTPS 反向代理到 core 的回调端口（默认 9003）。

回调同时校验签名、加密接收方及消息企业，覆盖客户、员工、部门、群和标签的变更。处理成功后保存事件回执；重复投递直接确认，失败保留重试机会。

## 服务商授权

运营方配置 `WecomProvider` 的 SuiteID、SuiteSecret、回调 Token/AES Key、ReceiveID 及固定 HTTPS RedirectURL。ReceiveID 必须与服务商平台加密回调的接收方标识一致，留空按 SuiteID 校验。

- 授权事件地址：`/wecom/provider/callback`，接收 suite_ticket、create_auth、change_auth、cancel_auth。
- 企业数据事件地址：`/wecom/provider/data`。
- 租户管理员或经理在渠道设置点击「服务商授权接入」。state 绑定发起租户和账号，授权返回后自动完成绑定。
- 永久授权码、suite_ticket 加密保存；suite token 与企业 token 分开获取。企业 token 按 corp 缓存，服务商模式不读取自建应用 Secret。
- 授权变更刷新应用信息；撤销授权停用该企业并停止存档，不影响其他企业。旧事件不能覆盖更新的授权状态。
- 未携带租户授权 state 的应用市场安装不会自动分配租户。自建应用与服务商模式之间也不会自动改绑已有企业数据。

## 会话存档

msg-server 使用与 core 相同的 PostgreSQL 数据库、`App.Key` 和非空 `App.InnerSrvAppCode`。由 core 统一执行数据库迁移，msg-server 的 `App.AutoMigration` 建议关闭。

在企业配置中单独开启会话存档，填写存档 Secret，以及「版本号 → PEM 私钥」的 JSON 对象。私钥支持 PKCS#1 和 PKCS#8 RSA 格式，并可同时保留多个版本用于轮换；查询接口只返回掩码。服务商授权不会代替存档凭据。

msg-server 每分钟发现启用存档的企业。每个企业使用独立 SDK、版本私钥和数据库游标；同一企业用数据库锁避免重叠同步。整个批次解密、存储成功后才提交游标，失败不会跳过消息。媒体对象按租户和企业分区，读取时重新签名下载地址。

内部 HTTP 请求签名覆盖时间戳、方法、路径、完整查询和请求体，包括 tenant 与 corp。存档服务还会重新检查企业归属。两端必须同时升级，旧的部分参数签名不再接受。

## 部署

1. 备份数据库，确认旧企微数据均有正确的企业与租户映射。
2. 在 scrm-server 中运行 `go run ./cmd/migrate -schema-only`，执行新增的 0027—0030 迁移。归属不明或冲突的数据会阻止迁移，需先修正。
3. 同时部署 core、gateway 和 dashboard；服务商模式需配置平台应用及上述回调。
4. 在 Linux amd64 上构建并部署 msg-server：`CGO_ENABLED=1 go build -o msg-arch-server .`，设置 `LD_LIBRARY_PATH` 包含项目 `lib` 目录。
5. 配置各企业凭据后，验证两企业相同外部 ID 的隔离、重复回调、授权撤销、私钥轮换及重启续传。

MySQL 存档实例不能直接继续连接旧库；需先把历史存档与对应客户、员工引用迁入 PostgreSQL，再启用该企业的存档任务。
