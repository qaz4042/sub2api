# my4g 部署说明

本文记录 my4g 的私有发布入口和最小运维检查。通用发布实现见
`deploy-systemd-release.sh`，主机参数集中在 `my4g.mk`；仓库不记录公网域名、凭证或生产快照。

## 环境契约

- SSH 别名：`my4g`
- 服务目录：`/opt/sub2api`
- 当前版本软链：`/opt/sub2api/current`
- 版本目录：`/opt/sub2api/releases`
- systemd 服务：`sub2api.service`
- 环境文件：`/opt/sub2api/sub2api.env`
- 发布文件 owner：`root:root`
- 远端本机健康检查：`http://127.0.0.1:8080/health`

systemd 的 `WorkingDirectory` 和 `ExecStart` 必须经由 `current` 指向当前版本目录，部署账号需有权
检查、重启服务并写入上述发布目录。

## 发布

```bash
# 前后端一起构建发布
make deploy-my4g

# 复用已有嵌入式前端 dist，仅重新构建后端
make deploy-my4g-backend-only
```

my4g 两个发布入口默认允许未提交改动，版本会带 `-dirty` 标记。需要校验工作区干净时，使用
`REQUIRE_CLEAN=1 make deploy-my4g`（仅后端入口同样支持）。通用发布脚本仍默认要求工作区干净。
发布前可执行不会构建、连接或上传的参数检查：

```bash
DRY_RUN=1 TAG=check make deploy-my4g
```

正式发布按“远端预检、前端构建、后端构建、上传 release、切换重启与本机健康检查”五步执行。
前端阶段分别记录依赖安装、类型检查和打包耗时；生产构建只执行一次类型检查，开发模式继续启用 checker。
成功时显示阶段与耗时，上传不输出 rsync 详细统计；任一构建步骤失败时才展开该步骤的完整输出。远端预检会确认
`releases/`、`current`、systemd service 和 `sub2api.env` 均已准备。

上传以开始上传时 `current` 指向的 release 为基准，先在远端复制到临时目录，再使用 rsync 校验和及压缩增量
传输原始二进制和 resources，并删除新版中已移除的文件。此方式兼容 macOS 自带的 openrsync。
新 release 不与旧版本共享硬链接；无需上一版
保留 gzip 文件。数据先写入独立 `.incoming-*` 目录，完整上传后沿用切换、健康检查和回滚流程。
收益取决于构建产物差异，首次或大改动仍可能接近全量传输。避免同时运行多个发布。

## 验证

```bash
ssh my4g 'readlink -f /opt/sub2api/current'
ssh my4g 'systemctl status sub2api --no-pager'
ssh my4g 'curl -fsS http://127.0.0.1:8080/health'
```

公网访问按需手工验证，不作为发布脚本的自动成功条件。

## 调试日志安全

生产环境不要长期启用 `SUB2API_DEBUG_GATEWAY_BODY`。该配置可能把请求和响应正文写入
`gateway_debug.log`，其中可能包含提示词、上传内容和模型响应。临时调试结束后应移除配置、重启服务，
并将已有日志权限收紧为 `0600`；不要把日志复制或提交到仓库。

## 回滚

新版本健康检查失败时，脚本会自动恢复发布前的 `current` 软链并重启服务。需要手工回滚时：

```bash
ssh my4g 'ls -1dt /opt/sub2api/releases/* | head'
ssh my4g 'ln -sfn /opt/sub2api/releases/<previous-release> /opt/sub2api/current && systemctl restart sub2api'
ssh my4g 'curl -fsS http://127.0.0.1:8080/health'
```

数据库迁移只向前执行。若旧版本与已执行迁移不兼容，应停止服务并恢复对应时间点的数据库备份，
不能只切换二进制版本。
