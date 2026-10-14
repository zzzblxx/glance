# 第三方组件与素材

第三方许可随源码及完整发布包保留。以下说明用于索引，不替代各组件原始许可证，
不改变各组件自身的授权条件。

| 内容 | 使用位置 | 许可文件 |
| --- | --- | --- |
| GNU Unifont 16.0.03 | 设备和 Mac 预览的点阵字库 | `assets/fonts/OFL-1.1.txt`、`unifont-COPYING.txt` |
| Go Mono | 管理页英文字体 | [Go 字体许可](Go-font-LICENSE.txt) |
| Go 模块 | 主机程序及测试 | [第三方许可目录](third-party/) 中各模块的 LICENSE / NOTICE |
| ArduinoJson、Seeed_GFX2 | 固件及共享绘图 | 同目录对应组件许可 |
| esptool 及 Python 依赖 | 随包刷写工具 | 同目录对应发行包许可 |
| PlatformIO、PyInstaller 等 | 构建工具 | 同目录对应工具许可及例外条款 |

Unifont 的来源、原文件及生成字库校验值见 `assets/fonts/README.md`。
本项目只重新打包点阵，不分发 macOS 专有字体。
四幅横幅由项目所有者提供，并随 Glance 自有代码和文档采用 [MIT License](../LICENSE)。
图片原图和生成的嵌入数据均包含在该授权范围内，来源与授权说明见 `assets/banners/README.md`；
这些图片不适用 Unifont 或 Go 字体的许可证。

`scripts/collect-notices.py` 从构建环境收集 Go、Python 和固件依赖的许可证。
因此目录包含构建期和测试期组件，并非每个组件都会进入运行时。
更新依赖时重新收集并检查版本、许可和必要来源要求，不能只删除看似未使用的声明。

项目 MIT 授权不替代本页列出的第三方许可证。分发源码或完整发布包时，应保留相应版权、许可和例外条款。
