# uclash-cli

`npm i -g uclash-cli` 安装 [uclash](https://github.com/IKEJAY-code/uclash) —— 在 Linux
服务器上以普通用户身份运行 mihomo（Clash.Meta）的进程管理器。安装后命令名为 `uclash`。

postinstall 行为：

- 按照本包版本（如 `v0.1.0`）下载对应的 GitHub Release 二进制
- 强制校验 SHA-256（校验值内置于 `install.js`，升级需发新版本）
- GitHub 直连失败时自动按 `gh-proxy.com`、`ghfast.top`、`ghproxy.net` 回退

环境变量（可选）：

- `UCLASH_GH_MIRROR`：自定义镜像前缀（空格分隔多个）
- `UCLASH_LOCAL_FILE`：从本地文件安装（离线场景）
- `UCLASH_VERSION` + `UCLASH_SHA256`：安装其他版本（必须显式给出校验值）
- `UCLASH_FORCE=1`：即使已存在也重新下载

安装完成后：

```bash
uclash init
uclash sub add "<订阅链接>"
eval "$(uclash proxy on)"
uclash ui
```

完整文档见主仓库 README；本项目不提供任何节点或订阅。
