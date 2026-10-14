# 安装与升级

## 环境与文件

- Apple Silicon Mac、E1003、支持数据的 USB 线；Mac 需要联网读取额度。
- 同一 macOS 用户下已安装 Codex CLI，并通过 `codex login` 登录。
- 完整包 `glance-macos-arm64.tar.gz` 和配套 `glance-macos-arm64.tar.gz.sha256`。
- 如没有 `/dev/cu.wchusbserial…`，按设备与 WCH 官方说明安装串口驱动，并在系统设置中允许驱动扩展。

全过程无需 `sudo`。不要手动删除已有安装，也不要同时运行其他串口监视器或服务实例。

## 1. 校验并解压

两个下载文件都放进 Downloads，在同一个终端执行：

```sh
cd "$HOME/Downloads" &&
shasum -a 256 -c glance-macos-arm64.tar.gz.sha256 &&
glance_release_dir="$(mktemp -d "$HOME/Downloads/glance-install.XXXXXX")" &&
tar -xzf glance-macos-arm64.tar.gz -C "$glance_release_dir" &&
cd "$glance_release_dir" &&
./bin/glance version
```

校验必须显示 `OK`。若失败，停止安装并重新取得正确文件；不要跳过校验。
解压目录只用于安装，长期程序位于 `~/Library/Application Support/Glance`。

## 2A. 已有安装：覆盖升级

先查看[发布说明](release.md)。本次 1.0.0 使用数据驱动的 ChatGPT 标题并加入头部分隔线，绘图兼容标识为
`glance-3`，从封版前的版本升级时必须同时更新服务和固件。
先用 `./bin/glance-device --list` 确认串口，替换下方示例端口再执行：

```sh
export PATH="$HOME/.local/bin:$PATH" &&
bash scripts/stop-service.sh &&
bash scripts/flash-e1003.sh /dev/cu.wchusbserial20120 &&
bash scripts/install.sh
```

刷写期间不要拔线。使用 115200 波特率，校验后仅写入随包四个镜像，不执行全片擦除。
已有 Mac 配对仍保留，不需要重复读取出厂备份。失败时不会继续下一步，请保留完整报错。
安装回滚不会自动回滚已经写入的设备固件。

安装脚本会归档原安装、保留配置并启动新服务，不要求重新登录或配对。
以后若发布说明明确注明固件兼容、仅更新 Mac 服务，可直接运行 `bash scripts/install.sh`；
不要自行跳过本次固件升级。

## 2B. 全新设备：备份、刷机、配对

只在尚未安装本项目固件的设备上使用此流程。

1. 使用 `./bin/glance-device --list` 确认端口，关闭其他串口程序。
2. 先读取芯片信息，确认设备和闪存容量：

```sh
./bin/esptool --chip esp32s3 --port /dev/cu.wchusbserial20120 flash-id
```

3. 确认为 32 MB 闪存后，保存完整出厂备份。若容量不同，不要照抄这个长度：

```sh
mkdir -p "$HOME/Library/Application Support/Glance Backups" &&
glance_factory_dir="$(mktemp -d "$HOME/Library/Application Support/Glance Backups/factory.XXXXXX")" &&
./bin/esptool --chip esp32s3 --port /dev/cu.wchusbserial20120 --baud 115200 \
  read-flash 0x0 0x2000000 "$glance_factory_dir/factory-32mb.bin" &&
shasum -a 256 "$glance_factory_dir/factory-32mb.bin"
```

全片读取可能约 50 分钟；成功前不要刷机。备份含设备原始数据，不上传仓库或分享给他人。

4. 确认允许覆盖出厂固件后，刷写、创建配置、配对、安装：

```sh
export PATH="$HOME/.local/bin:$PATH" &&
bash scripts/flash-e1003.sh /dev/cu.wchusbserial20120 &&
./bin/glance init-config &&
./bin/glance-device --port /dev/cu.wchusbserial20120 &&
bash scripts/install.sh
```

`init-config` 仅供首次创建配置，已有文件时不会覆盖；已有安装请走 2A。
配对只记录端口与设备身份，不发送账号凭据。设备之后不需要配置 Wi-Fi。

## 3. 检查安装

打开 `http://127.0.0.1:8888`：

- 连接卡显示 USB 已连接，预览与屏幕内容一致（设备应在首页）。
- 右上为日期＋星期，四张便签内容保留，输入框每行七个汉字。
- 修改一张便签并保存，预览和设备正常更新；测试后可改回。
- 同日内容不变时不再因为一分钟过去而闪屏；额度、状态或日期变化仍会刷新。

已安装程序的版本和后台状态可这样查看：

```sh
"$HOME/Library/Application Support/Glance/bin/glance" version
launchctl print "gui/$(id -u)/com.glance.companion"
```

首次全屏刷新可能闪黑，这是灰度刷新行为。升级后清理 Downloads 安装文件不影响运行；
不要同时清理出厂备份。升级归档在 `~/Library/Application Support/Glance Backups/`，包含配置和凭据。

## 日常维护

便签在网页保存后无需重启。其他配置修改后重启服务：

```sh
launchctl kickstart -k "gui/$(id -u)/com.glance.companion"
```

串口变化或更换设备时，在完整安装包目录执行停止服务、重新配对、安装：

```sh
bash scripts/stop-service.sh &&
./bin/glance-device --port /dev/cu.wchusbserial你的设备 &&
bash scripts/install.sh
```

不要把示例中的“你的设备”原样当作真实端口。

卸载：`bash scripts/uninstall.sh`。脚本停止服务并归档程序与启动项，保留便签、配置、
配对及运行数据；不会删除 Codex 登录数据。无需手动处理端口号。

## 常见问题

| 现象 | 检查 |
| --- | --- |
| `codex: command not found` | 新开终端，或先执行 `export PATH="$HOME/.local/bin:$PATH"`；确认 CLI 已安装并登录 |
| 找不到串口 | 确认是数据线、驱动扩展已启用；重插后重新列出端口 |
| 刷写中断 | 停止其他占串口程序，确认连接稳定并使用 115200；不要做全片擦除 |
| 未配对或版本不匹配 | 停服务后使用配对工具；版本不匹配需使用同一发布包的程序与固件 |
| 网页可用但屏幕不更新 | 查看 USB 状态和页码；检查配对端口、设备身份及当前是否在首页 |
| 暂无近期任务活动 | 当前只识别本机活动记录，不支持远程任务；与额度采集独立 |
| 同时存在两个安装目录 | 停止操作，先确认各自配置，不要强行合并或删除 |
| 需要查看启动错误 | 先停后台服务，再以前台方式运行安装目录中的 `bin/glance serve --config …/config.json`；不要同时启动两个实例 |

没有 Developer ID 签名或公证。如 macOS 阻止下载程序，确认来源后使用系统正常的允许打开流程，
不要全局关闭 Gatekeeper 或其他安全设置。
