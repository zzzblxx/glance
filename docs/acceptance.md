# 验证清单

自动测试不能代替 E1003 实机验收。以下命令在源码根目录运行；安装包使用者只需完成最后的实机检查。

## 自动测试

先按[构建文档](build.md)准备依赖，然后运行：

```sh
make test
PIO="$PWD/scripts/pio.sh" bash scripts/build-firmware.sh
bash scripts/package-release.sh
```

`make test` 构建本地程序，运行 Go race、Go vet、原生绘图 ASan/UBSan、
配置迁移、额度精度与过期状态、刷新去重、便签限制、USB 错误处理和隔离安装测试。
标题加粗测试验证新增黑色像素且不影响标题外区域；其他字号继续使用原有描粗参数。

## 网页检查

需要 Node.js 和 Playwright（含 Chromium）。使用独立测试配置，不连接硬件：

```sh
mkdir -p var/demo
bin/glance init-config --config var/demo/config.json
node scripts/dev-config.mjs var/demo/config.json
bin/glance serve --demo --config var/demo/config.json
```

配置已存在时跳过 `init-config`，不要用真实安装配置运行 `dev-config.mjs`。
另开终端在源码根目录运行：

```sh
node scripts/smoke.mjs var/demo/config.json
node scripts/test-note-layout.cjs
```

若 Playwright 不在默认模块路径，可设置 `PLAYWRIGHT_MODULE` 为其模块目录。
排版测试覆盖五种窗口宽度、四个输入框、中文、ASCII、中英混排、标点和手动换行；
截图输出到 `var/notes-editor-seven-columns.png`。排版测试不会保存修改。
完成后在服务终端按 Control-C 停止 demo。

## 发布包自检

```sh
(cd dist && shasum -a 256 -c glance-macos-arm64.tar.gz.sha256)
glance_check_dir="$(mktemp -d "$PWD/var/release-check.XXXXXX")" &&
tar -xzf dist/glance-macos-arm64.tar.gz -C "$glance_check_dir" &&
GLANCE_INSTALL_TEST=1 GLANCE_INSTALL_SOURCE="$glance_check_dir" \
  go test ./internal/installation -count=1
```

测试使用临时目录与替身启动服务，不改动真实登录项。还需确认：

- 包内 `VERSION` 与 `bin/glance version` 一致；四个固件 SHA256 全部通过。
- 固件与主机的绘图兼容标识一致。
- 包中没有账号、配置、设备备份、缓存、Git 历史或开发环境。
- README 的相对文档及图片链接存在；随包保留第三方许可。

## 实机覆盖升级验收

按[安装与升级](install.md)执行，不跳过固件兼容性说明。

- 原有便签、配对和配置保留；旧文件已归档到 `Glance Backups`。
- 8888 页面能打开，显示设备已连接；ChatGPT 大标题为本次加粗样式。
- 无 5 小时限制时只显示通栏 7 天；两个窗口存在时各占一半。
- 百分比、重置时间和重置机会与实际来源一致；缺失信息不伪造。
- 四个输入框一行七个汉字、最多四行；28 字可保存，超限明确报错。
- 保存后设备更新；重新打开管理页仍能看到保存的内容。
- 数据和状态不变时不每分钟闪屏；发生实际变化后自动更新。
- 暂时断开再接入 USB 后恢复同步；按键翻页可用。
- 退出登录再登录后服务自动运行，删除 Downloads 解压目录不影响运行。

这里的清单不是实机通过记录。自动测试无法确认真实线缆、驱动、墨水屏残影或设备供电表现。
