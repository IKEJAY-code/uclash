# uclash

在 Linux 服务器上以**普通用户**身份运行 mihomo（Clash.Meta）内核。

不需要 sudo、不需要 systemd、不需要 tmux，不写 `/etc`，不碰 iptables。
每个用户拥有完全独立的进程、端口、密钥、配置和日志，互不干扰，
特别适合多人共用的服务器 / 集群 / 容器 / WSL 环境。

```
# 主源（GitHub）
~$ curl -fsSL https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh
# 国内网络（镜像）
~$ curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh
~$ uclash init
~$ uclash sub add "https://your-airport.example/sub?token=..."   # Clash YAML 或 base64 节点链接均可
~$ uclash start && proxyon
~$ uclash ui          # 打印带密钥的面板地址，VS Code 终端里可直接 Ctrl+点击打开
```

## 特性

- **用户态一键安装**：单个静态二进制，装进 `~/.local/bin`（类 npm 前缀），GitHub 官方源 + 国内加速镜像自动回退
- **无系统依赖**：`setsid` 自管守护进程 + PID 文件 + flock，WSL、Docker、无 systemd 的 HPC 都能用
- **多用户安全**：
  - 所有文件都在 `$HOME` 下；端口启动时探测空闲值，被别人占用会自动重选
  - `uclash stop` 会先用 `/proc/<pid>/cmdline` 校验“这是我自己启动的 mihomo”，绝不误杀他人进程
  - 只监听 `127.0.0.1`，`allow-lan: false`，不做 TUN（无 root 本来也做不了）
- **订阅全格式**：Clash/Mihomo YAML 直接使用；**base64 / 纯文本节点链接自动转换**为 Clash 配置
  （支持 vmess、vless（含 reality）、trojan、ss（含 obfs/v2ray-plugin）、ssr、hysteria2、hysteria、tuic、socks5）
- **内置网页面板**：mihomo 自己托管 metacubexd，一个端口同时是 REST API 和 UI；
  网页里可以切策略组、换端口、改 rule/global/direct 模式、看连接与日志
- **命令行切节点**：`uclash node ls / use / test`，不打开网页也能选节点、测延迟
- **顺手拿地址**：`uclash ui` 输出 `http://127.0.0.1:<port>/ui/#/setup?hostname=...&port=...&secret=...`，
  VS Code Remote 终端会自动识别 localhost 链接并提供端口转发，Ctrl+点击即开
- **终端开关**：`proxyon` / `proxyoff`（安装进 rc 的 shell 函数），或 `eval "$(uclash env on)"`，
  只影响当前 shell，不影响其他终端和其他用户

## 安装

### 方式一：curl | sh（推荐）

```sh
# 主源（GitHub）
curl -fsSL https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh

# 国内网络：给 raw 链接加镜像前缀
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/IKEJAY-code/uclash/main/scripts/install.sh | sh
```

装到 `~/.local/bin/uclash`。**二进制下载本身也会自动回退**：GitHub 直连失败时
依次尝试 `gh-proxy.com`、`ghfast.top`、`ghproxy.net`；可用 `UCLASH_GH_MIRROR`
指定自有镜像（空格分隔多个前缀）。

完全离线或走内网镜像时：

```sh
# 浏览器下载 uclash-linux-amd64 后本地安装
UCLASH_LOCAL_FILE=./uclash-linux-amd64 sh install.sh
# 指向内网静态镜像
UCLASH_DOWNLOAD_URL=https://your.mirror/v0.1.0/uclash-linux-amd64 sh install.sh
```

### 方式二：npm（可选，服务器需要 Node）

```sh
npm i -g uclash-cli   # 安装后命令名为 uclash；下载与包版本一致的二进制并校验 SHA-256
```

> npm 上的 `uclash` 名为他人抢注的空包（无任何版本），因此包名为 `uclash-cli`，
> 安装后的命令仍是 `uclash`。包会同步到 npmmirror（淘宝源），国内 CDN 直连；
> 也可用 `UCLASH_GH_MIRROR` 指定镜像。

### 方式三：源码构建

```sh
git clone https://github.com/IKEJAY-code/uclash && cd uclash
make linux            # 产出 dist/uclash-linux-amd64、dist/uclash-linux-arm64
```

> Gitee 镜像因平台内容审查（raw 文件返回 HTTP 451）已不可用，主源为 GitHub。
> 实验室内网可自建静态镜像，通过 `UCLASH_DOWNLOAD_URL`、`uclash init --core/--ui`
> 与 lab-scripts 工具的 `--base-url` 复用同一批二进制（SHA-256 不变）。

## 快速开始

```sh
# 1. 初始化：下载 mihomo 内核 + metacubexd 面板，自动分配空闲端口，写入 shell 助手
uclash init
#    下载慢/被墙：uclash init --mirror https://gh-proxy.com
#    用现成的内核/面板：uclash init --core ./mihomo --ui ./metacubexd-dir

# 2. 添加订阅（Clash YAML 或 base64 节点链接都行）
uclash sub add "https://airport.example/api/v1/client/subscribe?token=..."

# 3. 启动 + 在当前 shell 开代理
uclash start
proxyon            # 或者： eval "$(uclash env on)"

# 4. 打开面板（网页里换策略组 / 换端口 / 切模式），或命令行切节点
uclash ui
uclash node ls && uclash node use <名字>
```

## 命令参考

| 命令 | 说明 |
| --- | --- |
| `uclash init` | 初始化：内核、面板、端口、secret、shell 助手 |
| `uclash start / stop / restart` | 控制本用户的 mihomo 进程（只动自己的） |
| `uclash status [--quiet]` | 进程、模式、端口、面板地址、订阅更新时间 |
| `uclash sub add <url> [--name N]` | 添加订阅（YAML 或 base64 链接，自动识别转换） |
| `uclash sub ls / use <N> / update [N\|--all] / rm <N>` | 订阅列表 / 切换 / 更新 / 删除 |
| `uclash sub auto <小时>` | 自动更新间隔（每次 `start` 时检查一次） |
| `uclash profile import <file.yaml> [--name N]` | 导入本地 Clash YAML（完整配置用这个） |
| `uclash node ls [group]` | 列出策略组 / 某组内的节点（带当前选择与延迟） |
| `uclash node use [group] <名字\|序号>` | 切换节点（默认组优先 PROXY，支持组内序号） |
| `uclash node test [group]` | 并发测速组内所有节点，按延迟排序 |
| `uclash mode [rule\|global\|direct]` | 查看 / 切换路由模式（运行中热切换） |
| `uclash port [set mixed\|controller <n>]` | 查看 / 修改端口（controller 变更会重启内核） |
| `uclash env on\|off [--sh]` | 输出设置/清理终端代理变量的 shell 语句 |
| `uclash ui [--open] [--plain]` | 打印（或打开）面板地址 |
| `uclash log [-n N] [-f]` | 查看/跟踪内核日志 |
| `uclash core info / core update` | 查看 / 升级 mihomo 内核（支持 `--mirror`） |
| `uclash shell install / uninstall / status` | 管理 rc 文件里的 `proxyon`/`proxyoff` |
| `uclash doctor` | 自检：目录权限、内核、端口、订阅、PATH 等 |
| `uclash uninstall [--purge --yes]` | 停止并清理 shell 集成；`--purge` 删除数据 |

## 文件位置

```
~/.config/uclash/config.yaml      # 用户配置（端口、模式、镜像、自动更新等，可手改）
~/.local/share/uclash/
├── bin/mihomo                    # 内核（uclash core update 管理）
├── ui/                           # metacubexd 面板静态文件
├── profiles/                     # 各订阅/导入的 YAML + registry.yaml
├── config.yaml                   # 由 profile + 本地覆盖生成的运行时配置
├── run/mihomo.pid + uclash.lock  # 进程管理
└── logs/mihomo.log               # 内核日志
```

也可以整体搬走：`UCLASH_HOME` 覆盖数据目录，`UCLASH_CONFIG` 覆盖配置文件路径，
`--data-dir` / `--config` 是等价命令行参数（同一台机器跑多实例/测试时很有用）。

## 本地覆盖都做了什么

订阅 YAML 会被原样保留，只做这些必要覆盖，让内核“老实”地跑在用户态：

- `mixed-port` / `external-controller` / `secret` / `external-ui`：指向本用户的端口与面板
- `allow-lan: false`、`bind-address: 127.0.0.1`：只监听本机回环
- 删除 `port` / `socks-port` / `redir-port` / `tproxy-port` / `tun` / `dns.listen` / `external-ui-url`：
  这些要么额外监听不受控端口，要么需要 root
- `mode` / `log-level` 使用 uclash 配置里的值；补 `profile.store-selected: true` 记住手动选的节点
- 配置了 `--mirror` 时，同时把 `geox-url` 指向镜像，GeoIP/GeoSite 也能顺利下载

## 订阅转换说明

当订阅返回的不是 Clash YAML（常见为 base64 节点链接），uclash 会自动转换：

- 自动识别 base64（标准/URL-safe/无 padding）与纯文本链接列表
- 支持协议：`vmess`（JSON 与 legacy）、`vless`（含 `reality-opts`、`flow`）、`trojan`、
  `ss`（SIP002、legacy、`obfs`/`v2ray-plugin`）、`ssr`、`hysteria2`/`hy2`、`hysteria`、`tuic`、`socks5`
- 生成的配置只含：节点 + `PROXY`(select) + `AUTO`(url-test) + 最小规则（`GEOIP,LAN,DIRECT` / `MATCH,PROXY`），
  `uclash sub ls` 中会标记 `sub(conv)`
- 想要完整分流规则，请用 `uclash profile import` 导入自己的 YAML

## 常见问题

**订阅报错说不认识？**
把完整报错贴出来。uclash 会尽力识别 YAML / base64 / 纯链接；如果是个别畸形链接，
转换时会跳过并继续，只要求至少一个节点可用。

**GitHub 下载失败/太慢？**
`install.sh` 会自动按内置镜像列表重试；仍失败时用
`UCLASH_GH_MIRROR=https://gh-proxy.com sh install.sh` 指定镜像，
或浏览器下载二进制后用 `UCLASH_LOCAL_FILE=... sh install.sh`。
`uclash init` 下载内核/面板同理：`uclash init --mirror https://gh-proxy.com`。

**新增的订阅什么时候被内核校验？**
只有被切换为 active（`uclash sub use`）或下次 `uclash start` 时才会真正加载到 mihomo。
如果内核拒绝该配置，uclash 会回滚运行时配置与 active 选择并报错，坏配置不会让服务起不来；
每个 profile 在更新前都会留一份 `profiles/<名字>.yaml.bak`，可随时手工恢复。

**和系统里已有的 Clash 冲突吗？**
不会。uclash 只挑当前空闲端口；如果启动时发现端口已被别人占用，会自动重选一组并写回配置。

**面板打不开？**
`uclash ui` 打印的地址只能在能看到该服务器 `127.0.0.1` 的地方访问。
用 VS Code Remote 时点终端里的链接即可自动转发；纯 SSH 可以用
`ssh -L <本地端口>:127.0.0.1:<controller端口> user@host`。

**为什么不用 systemd --user？**
很多服务器/容器/WSL 没有可用的 user session（本项目的 WSL 测试环境里
`systemctl --user is-system-running` 就是 `offline`）。uclash 用 setsid 自行守护，
有 systemd 与否都能用；进程管理、日志、自启控制都在 `uclash` 命令里。

## 发布与开发

```sh
go test ./...                       # 单元测试（含订阅转换的协议用例）
make linux                          # 交叉编译 linux amd64/arm64
scripts/release.sh v0.1.0           # 产出全部 4 个平台资产 + SHA256SUMS
git tag v0.1.0 && git push origin v0.1.0   # 推送 tag，Actions 自动发布 Release
# 本地直接发布（需 gh CLI 已登录）：
# scripts/release.sh v0.1.0 --publish        （重建并发布）
# scripts/release.sh v0.1.0 --no-build --publish （发布 dist/ 中的现有文件）
scripts/wsl-run.sh                  # 仅在 Windows 开发时：拷入 WSL 跑集成测试
```

集成测试 `scripts/wsl-test.sh` 完全离线运行（本地 HTTP 订阅 + 本地内核/面板），
覆盖 69 项断言：初始化、YAML 订阅合并、**base64 订阅转换**（vmess/vless reality/trojan/ss/hysteria2）、
启动/API/面板、模式与端口热切换、`node ls/use/test`、env 输出、profile 导入、
**坏配置热重载回滚**、**双实例隔离**（两个“用户”互不影响）以及 **stop 防误杀**（伪造 PID 文件不会被杀）。

## 第三方组件与许可

uclash 以 **MIT** 发布（[LICENSE](LICENSE)）。它只做用户态进程管理，**不打包、不修改、
不分发**任何内核/面板/节点数据；mihomo（GPL-3.0）与 metacubexd（MIT）等组件由用户在
运行时从上游官方渠道获取，以独立进程方式调用。完整的组件、版本、许可与出处见
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

> 本项目不提供任何代理服务、节点或订阅，也不包含任何“翻墙”能力；是否使用及如何使用由用户自行决定并自负责任。

## Roadmap

- [ ] 可选 `systemctl --user` 单元生成（有 user session 时用 systemd 托管）
- [ ] 面板内导入订阅（uclash 本地辅助服务）
- [ ] `uclash node test` 结果写入面板缓存 / 定时自动切换最优节点
- [ ] macOS 完整支持（代码已按平台分层，等待实机验证）
