# Glance 1.0.0

## 中文

一览，桌面电子纸 AI 额度看板。首个正式版本，发布于 2026-10-14。
目前支持 Apple Silicon Mac、OpenAI 额度与 Seeed Studio reTerminal E1003。
界面显示 ChatGPT，数据通过 Codex CLI 获取，不代表所有 ChatGPT 功能的统一余额。

### 主要功能

- 一根 USB 线供电并传输结构化数据，设备自行绘图，无需 Wi-Fi 或 TF 卡。
- 1404 × 1872 竖屏、16 级灰度，显示日期和星期；内容变化才刷新，不按分钟强制刷屏。
- 显示 5 小时、7 天额度的整数剩余百分比、进度条和重置时间；只有 7 天限制时占满整行。
- 显示可用重置次数和已知最近到期时间；根据剩余额度与重置次数切换四档内置横幅。
- 在 `http://127.0.0.1:8888` 管理四张便签，每张四行、每行七个汉字，保存时校验容量。
- 登录自动启动；升级保留便签、配对和配置，旧安装自动归档。
- 自有代码、文档和四张横幅采用 MIT，署名 zzzblxx；第三方组件与字体保留原许可证。

### 下载与安装

下载本 Release 附件中的 **`glance-macos-arm64.tar.gz`** 和
**`glance-macos-arm64.tar.gz.sha256`**，先校验，再解压安装。
GitHub 自动生成的 Source code 文件不是安装包。Mac 需预先安装并登录 Codex CLI。

[中文安装步骤](https://github.com/zzzblxx/glance/blob/v1.0.0/docs/install.md)
包含首次备份、刷写、配对、覆盖升级及卸载流程。
从封版前的版本升级时，按“停止服务 → 刷写随包固件 → 安装服务”执行，
主机与固件必须同时更新到绘图兼容标识 `glance-3`。
无需手动删除旧安装或再次读取已有的出厂备份；安装回滚不会回滚已写入的固件。

### 已知边界

当前界面与图片文字为中文；不采集远程 Codex 任务活动，尚未实现 Anthropic、其他屏幕、
触控或局部刷新。灰度全屏刷新会短暂闪黑。管理页仅对本机开放。

发布前已运行 Go 测试、共享绘图测试、原生绘图检查、隔离安装和管理页检查；
实机显示、连接恢复及登录启动仍需在目标设备上按
[验证清单](https://github.com/zzzblxx/glance/blob/v1.0.0/docs/acceptance.md)确认。

## English

Glance is a desktop e-paper dashboard for AI service quotas and personal notes.
This is the first stable release, published on October 14, 2026. It supports
Apple Silicon Macs, OpenAI quota data and the Seeed Studio reTerminal E1003.
The UI uses the name ChatGPT, but data is retrieved through Codex CLI and does
not represent a single allowance for every ChatGPT feature.

### Highlights

- One USB cable for power and structured data; the device renders locally.
  No device Wi-Fi or microSD card is required.
- A 1404 × 1872 portrait display with 16 grayscale levels, date and weekday.
  Content-driven refresh avoids unnecessary minute-by-minute screen flashes.
- Whole-number remaining percentages, progress bars and reset times for the
  5-hour and 7-day windows. A weekly-only quota fills the row.
- Available reset counts and the nearest known expiration, plus four built-in
  banners selected by remaining quota and available resets.
- Four editable notes at `http://127.0.0.1:8888`, each with four rows of seven
  full-width characters. Layout capacity is checked before saving.
- Launch at login, with notes, pairing and configuration preserved during
  upgrades and the previous installation archived.
- MIT-licensed project code, documentation and all four banners, credited to
  zzzblxx. Third-party dependencies and fonts retain their original licenses.

### Download and install

Download **`glance-macos-arm64.tar.gz`** and
**`glance-macos-arm64.tar.gz.sha256`** from this release's assets. Verify the
checksum before extracting and installing. GitHub's automatic Source code
archives are not installation packages. Install and authenticate Codex CLI on
the Mac first.

The [English installation guide](https://github.com/zzzblxx/glance/blob/main/README.en.md#installation-and-upgrades)
covers factory backup, flashing, pairing, upgrades and uninstalling.
When upgrading from a pre-release build, **stop the service → flash the bundled
firmware → install the service**. Update both the host and firmware to renderer
compatibility ID `glance-3`. Do not manually delete an existing installation;
there is no need to repeat an existing factory backup. Installer rollback does
not restore firmware already flashed to the device.

### Limitations and verification

The current UI and banner captions are in Chinese. Remote Codex task activity,
Anthropic, other displays, touch input and partial refresh are not implemented.
Grayscale full-screen updates briefly flash black. The management page is
accessible only from the local Mac.

Pre-release checks included Go tests, shared-renderer tests, native rendering
checks, isolated installation tests and management-page checks. Actual display
behavior, reconnection and launch at login still need validation on the target
hardware; see the [acceptance checklist (Chinese)](https://github.com/zzzblxx/glance/blob/v1.0.0/docs/acceptance.md).

## License / 许可

[MIT License](https://github.com/zzzblxx/glance/blob/v1.0.0/LICENSE)
· Copyright (c) 2026 zzzblxx
· [Third-party notices / 第三方说明](https://github.com/zzzblxx/glance/blob/v1.0.0/docs/third-party.md)
