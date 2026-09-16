# 第三方组件与许可（Third-Party Notices）

uclash 自身以 **MIT** 许可发布（见 [LICENSE](LICENSE)）。它是一层"用户态进程管理器"：
**不打包、不修改、不分发**任何代理内核、面板或节点数据，所有外部组件都由
用户在运行时从上游官方渠道自行获取，在此向各上游项目致谢并声明其许可。

## 运行时依赖（由 uclash 下载或调用，不属于本仓库产物）

| 组件 | 用途 | 许可 | 上游 |
| --- | --- | --- | --- |
| mihomo (Clash.Meta) | 代理内核，由 `uclash init` / `uclash core update` 从官方 Release 下载，以独立子进程方式运行（不链接、不修改、不再分发） | GPL-3.0 | <https://github.com/MetaCubeX/mihomo> |
| metacubexd | Web 面板静态文件，由 `uclash init` 下载 gh-pages 构建产物用于 `external-ui` | MIT (© 2023 MetaCubeX) | <https://github.com/MetaCubeX/metacubexd> |
| meta-rules-dat GeoIP/GeoSite | 内核运行期自行下载的路由数据（`geox-url`），可选且可替换 | 遵循上游仓库声明 | <https://github.com/MetaCubeX/meta-rules-dat> |

> 关于 GPL：uclash 与 mihomo 之间只通过命令行参数、配置文件与 HTTP API 交互，
> 属于"独立进程调用"，不构成衍生作品；mihomo 二进制及其 GPL 义务完全由用户
> 从上游下载时适用。uclash 的源码与发布产物中不包含 mihomo 代码或二进制。

## 编译期依赖（Go 模块，静态链接进 uclash 二进制）

| 组件 | 许可 | 上游 |
| --- | --- | --- |
| spf13/cobra | Apache-2.0 | <https://github.com/spf13/cobra> |
| spf13/pflag | BSD-3-Clause | <https://github.com/spf13/pflag> |
| inconshreveable/mousetrap | Apache-2.0 | <https://github.com/inconshreveable/mousetrap> |
| gopkg.in/yaml.v3 | MIT + Apache-2.0 (© Canonical Ltd / Kirill Simonov) | <https://github.com/go-yaml/yaml> |

上述模块的许可证全文可在 `go.sum` 所列模块源码中查看。

## 明确声明

- uclash **不提供**任何代理服务、节点、订阅或翻墙能力；是否使用、如何使用由用户自行决定并自负责任。
- 引用、借鉴的上游项目仅限本文件所列；若发现遗漏，请提 Issue 补充。
