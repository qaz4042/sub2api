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

发布默认拒绝 dirty 工作区。仅在明确需要验证未提交改动时，才显式使用
`REQUIRE_CLEAN=0`。发布前可执行不会构建、连接或上传的参数检查：

```bash
DRY_RUN=1 REQUIRE_CLEAN=0 TAG=check make deploy-my4g
```

正式发布按“远端预检、前端构建、后端构建、上传 release、切换重启与本机健康检查”五步执行。
成功时仅显示阶段与耗时；任一构建步骤失败时才展开该步骤的完整输出。远端预检会确认
`releases/`、`current`、systemd service 和 `sub2api.env` 均已准备。

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
