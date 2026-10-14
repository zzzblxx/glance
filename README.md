# Glance

简体中文 | [English](README.en.md)

把 AI 服务额度和四张便签放到桌上的电子纸看板。

Glance 在 Apple Silicon Mac 上运行，通过一根 USB 线给 Seeed Studio reTerminal E1003
供电并传输数据。设备自行绘制画面，无需 Wi-Fi 或 TF 卡；Mac 查询额度仍需联网。

当前实现 **OpenAI（界面显示 ChatGPT，通过 Codex CLI 读取额度）＋ E1003**。
数据来源与设备适配分别组织，便于后续扩展；尚未实现 Anthropic 或其他屏幕。
ChatGPT 是展示名称，当前读到的是接口返回的具体额度池，并不代表所有 ChatGPT 功能的统一余额。

## 当前功能

- **额度看板**：显示 5 小时、7 天窗口的剩余百分比、进度条和重置时间；仅有 7 天限制时铺满整行。
- **重置机会**：显示可用次数及已知最近到期时间；信息缺失或不完整会明确提示。
- **四张便签**：在 `http://127.0.0.1:8888` 编辑，每张 4 行、每行 7 个汉字；英文数字占半字宽。
- **四档横幅**：根据剩余额度和重置机会自动切换设备内置图片，不通过 USB 传图。
- **按变化刷新**：1404 × 1872 竖屏、16 级灰度，右上显示日期与星期。内容不变不因分钟流逝刷屏；跨天或内容变化才生成新画面。
- **本机服务**：登录自动启动，配置和便签在升级后保留，失败时保留最近数据并标注状态。

## 显示预览

以下四张图由设备共用的绘图代码生成，使用固定示例数据，不是实际账号数据或实机照片。
每张同时覆盖多种状态：双额度／仅 7 天、四档横幅，以及有／无重置机会。

| 仅 7 天 · 有 2 次重置 · 站起来蹬 | 5 小时＋7 天 · 无重置 · 定速巡航 |
| --- | --- |
| ![仅 7 天剩余 79%，2 次重置，站起来蹬](docs/assets/dashboard.png) | ![双额度，5 小时剩余 50%，无重置，定速巡航](docs/assets/dashboard-cruise.png) |
| **5 小时＋7 天 · 无重置 · 要吃辣堡** | **5 小时＋7 天 · 无重置 · 玛卡巴卡** |
| ![双额度，5 小时剩余 20%，无重置，要吃辣堡](docs/assets/dashboard-snack.png) | ![双额度，5 小时剩余 0%，无重置，玛卡巴卡](docs/assets/dashboard-sleep.png) |

横幅分档采用“优先 5 小时，否则 7 天的剩余百分比 ＋ 可用重置次数 × 100”：
≥70 为站起来蹬，≥30 且 <70 为定速巡航，≥1 且 <30 为要吃辣堡，<1 为玛卡巴卡。
重置机会只影响选图分数，不加到进度条中。点开图片可查看原始分辨率。

右下“更新于”是本机生成画面的时间，不是额度最近查询时间，也不是设备成功刷新的回执。
网页预览是待同步的首页数据，不是屏幕截图；连接状态在管理页单独显示。

## 安装与升级

版本：1.0.0。发布包见[仓库 Releases](https://github.com/zzzblxx/glance/releases)。

支持 Apple Silicon macOS 与 E1003，需事先在同一用户下安装并登录 Codex CLI。
完整发布包统一命名为 **`glance-macos-arm64.tar.gz`**，配套 `.sha256` 校验文件。
应用版本在包内 `VERSION`，也可运行 `bin/glance version` 查看。

首次安装需要备份设备、刷固件、配对和安装服务。已有安装的覆盖升级保留便签、配对和配置。
是否需要刷固件取决于绘图兼容标识，不能只凭压缩包文件名判断。

**完整操作步骤：[安装与升级](docs/install.md)。本次版本：[发布说明](docs/release.md)。**

| 内容 | 位置 |
| --- | --- |
| 程序、配置与数据 | `~/Library/Application Support/Glance/` |
| 本机管理页 | `http://127.0.0.1:8888` |
| 登录启动服务 | `com.glance.companion` |
| 升级及卸载备份 | `~/Library/Application Support/Glance Backups/` |

安装包可在 Downloads 中解压，安装完成后不再依赖该目录。无需放进 Documents 或 Applications。

## 便签编辑

![四张便签输入框](docs/assets/notes-editor.png)

输入框固定七个汉字宽、四行高，支持留空和手动换行。自动折行不修改原文；手动换行也占行。
超过容量时不能保存。保存后自动同步，断线时可以先编辑，重新连接后再更新屏幕。

## 实现与边界

Go 负责采集、归一化和管理服务；USB 只传结构化 JSON；ESP32-S3 使用 C++ 在设备上绘图。
本地预览与 E1003 共用绘图代码和内置字体。当前数据由 OpenAI 的 Codex app-server 提供，
不包含其他平台账单或费用统计。服务商标识、界面名称和读取接口分别维护，标题由数据传入。

- 任务详情依赖本机可验证的活动记录，**不采集远程 Codex 任务活动**；没有记录不代表额度或 USB 故障。
- 支持设备白色按键翻页；未接入触控、局部刷新或实时分钟时钟。
- 当前使用内置 Unifont 点阵字库；大多数 emoji 等非 BMP 字符不支持。
- Mac 睡眠或设备离线时不阻止睡眠，屏幕保留旧画面；离线画面不能证明数据仍然新鲜。
- 管理页只对本机开放，不提供局域网或公网管理。USB 连接属于受信任的物理边界。

## 开发文档

| 文档 | 内容 |
| --- | --- |
| [安装与升级](docs/install.md) | 首次安装、更新、配对、故障排查和卸载 |
| [构建与发布](docs/build.md) | 工具版本、源码构建、测试和打包 |
| [项目结构与扩展](docs/architecture.md) | 数据来源、设备、传输和布局的边界 |
| [配置与数据安全](docs/configuration.md) | 配置字段、存储和安全边界 |
| [显示布局](docs/styling.md) | 字体、便签、横幅规则和刷新条件 |
| [USB 协议](docs/protocol.md) | 数据模型、握手、校验和传输约束 |
| [验证清单](docs/acceptance.md) | 自动测试与实机检查 |

```text
cmd/                              Go 程序入口和素材生成工具
internal/providers/openai/codex/   OpenAI 的 Codex 读取实现
internal/devices/e1003/            E1003 参数与本地预览适配
internal/transport/serial/         不绑定设备型号的串口传输
internal/protocol/                 显示数据协议与当前页面数据投影
internal/                         调度、模型、配置、管理页、存储与测试
firmware/e1003/                    E1003 硬件程序、布局和独立构建配置
firmware/shared/                   实际复用的生成素材
assets/fonts/                     公共字库及许可
assets/banners/                   公共横幅原图
native/e1003/                     E1003 的 Mac 预览绘图助手与测试
scripts/                          构建、安装、发布和验证脚本
artifacts/e1003/                   可刷写固件及校验值
docs/                             使用、维护文档、预览和第三方许可
VERSION                           应用版本
```

`bin/`、`dist/`、`var/` 和工具缓存是本地产物，不提交 Git。
运行 `make test` 前先按[构建文档](docs/build.md)准备依赖。

## 许可

Glance 自有代码、文档及四张横幅图片采用 [MIT License](LICENSE)，
Copyright (c) 2026 zzzblxx。允许使用、修改、分发和商用，须保留版权与许可声明。
图片原图及其生成的嵌入数据均适用这一授权，见 [横幅说明](assets/banners/README.md)。

第三方组件与字体仍遵循各自许可证，不因项目采用 MIT 而改变；详见 [第三方说明](docs/third-party.md)。
