# 构建与发布

## 工具与依赖

源码构建在 Apple Silicon macOS 上进行，需要 Go 1.24+、Xcode Command Line Tools、
Python 3 和 PlatformIO。普通使用者下载完整包即可，不需要这些编译工具。

```sh
python3 -m venv .venv-build
.venv-build/bin/python -m pip install -r requirements-build.txt
bash scripts/pio.sh pkg install -d firmware/e1003
```

Go 依赖锁定于 `go.mod` / `go.sum`。Python 构建工具直接依赖版本在 `requirements-build.txt`；
其传递依赖由安装器解析，不承诺不同工具环境产出逐字节相同的压缩包。
固件依赖由 `firmware/e1003/platformio.ini` 固定：

- pioarduino 55.03.311 / Arduino ESP32 3.3.11。
- Seeed_GFX2 提交 `8d7d14d4e2899a9451fef319035bad8eba1d3213`。
- ArduinoJson 7.4.2。

## Mac 程序

```sh
make build
bin/glance version
```

输出 `bin/glance`、`bin/glance-device`、`bin/glance-render` 和 `bin/unifont16.bin`。
应用版本来自根目录 `VERSION`；直接 `go run` 未注入版本时显示开发标识。
USB 本地预览不依赖系统中文字体；网页输入框使用系统中文字库和随程序提供的半字宽英文字体。

绘图助手与固件共享 `firmware/e1003/include/dashboard.h`，用 `native/e1003/preview.cpp` 生成 PNG。
`CXX`、`SDKROOT`、`ARDUINOJSON_DIR` 可覆盖编译环境路径。

## 固件

```sh
PIO="$PWD/scripts/pio.sh" bash scripts/build-firmware.sh
```

可以通过 `PLATFORMIO_CORE_DIR` 指定本地工具缓存，例如 `"$PWD/var/platformio"`。
使用该覆盖时，依赖安装与编译都应使用同一个值。`PIO` 也可指向自行安装的 PlatformIO。

固件使用 ESP32-S3、OPI PSRAM 和 8 MB 分区配置，程序分区大小为 3,342,336 字节。
分区配置不是物理闪存容量；刷写前应读取设备报告。外接串口为 UART1 GPIO44/43。
构建脚本设置 `SOURCE_DATE_EPOCH` 并生成 `artifacts/e1003/` 的四个镜像及 `SHA256SUMS`。
保留设备其他数据时使用随包刷写脚本，不使用合并的 factory 镜像或全片擦除。

## 字体和图片

```sh
python3 scripts/build-font.py
go run ./cmd/build-banners
```

字库由已固定校验值的 Unifont 源文件生成，见 `assets/fonts/README.md`。
图片生成器只接受 704 × 176 原图，不裁剪或缩放，生成 Gray4 RLE 头文件；修改后应检查生成差异。
字库与素材均嵌入设备，不需要 TF 卡，USB 不发送图片或字形。

## 测试

```sh
make test
```

包含 Go race、共享绘图、隔离安装、原生 ASan/UBSan、Go vet 和启动项生成检查。
浏览器和完整发布包检查见[验证清单](acceptance.md)。隔离安装测试使用临时目录和假的
`launchctl`，不会安装真实后台服务；不要在日常安装中设置 `GLANCE_INSTALL_HOME`。

## 独立刷写工具

在 Apple Silicon 上构建随包 esptool：

```sh
.venv-build/bin/python -m PyInstaller --noconfirm --onefile --name esptool \
  --distpath bin --workpath var/esptool-build --specpath var \
  --collect-all esptool scripts/esptool-entry.py
```

更改依赖后运行 `.venv-build/bin/python scripts/collect-notices.py`，核对第三方许可。
Go 字体许可在 `docs/Go-font-LICENSE.txt`，不能因为界面只有中文就删除其英文数字字体许可。

## 发布包

确认测试通过、固件是当前源码生成的、`bin/esptool` 已准备后：

```sh
make build
bash scripts/build-previews.sh
bash scripts/package-release.sh
```

输出：

```text
dist/glance-macos-arm64.tar.gz
dist/glance-macos-arm64.tar.gz.sha256
```

脚本重新构建 Mac 程序，验证四个镜像校验值以及主机、源码和固件的绘图兼容标识，
只打包明确列出的程序、固件、安装脚本、文档、版本文件和许可证。
不会包含 `var/`、配置、账号、出厂备份、源码工具环境或 Git 历史。

应用版本与绘图兼容标识分别维护：修改 `VERSION` 发布应用版本；绘图或设备数据格式改变时，
同步修改固件与 `internal/transport/serial/client.go` 的 `RendererVersion` 并重建固件。
仅网页调整不需要改变设备兼容标识。发布文件名不包含内部绘图编号。

打包不会自动提交、推送、创建标签或发布 GitHub Release。`dist/` 不提交 Git；
远端发布时将归档与校验文件作为同一版本的附件，不覆盖其他版本的资产。
