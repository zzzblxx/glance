# Glance

[简体中文](README.md) | English

An e-paper dashboard for AI service quotas and four personal notes.

Glance runs on an Apple Silicon Mac and connects to a Seeed Studio reTerminal
E1003 with a single USB cable for power and data. The device renders its own
screen: no Wi-Fi or microSD card is needed. The Mac still needs internet access
to retrieve quota data.

Currently supported: **OpenAI (displayed as ChatGPT, with quota data read through
Codex CLI) and E1003**. Provider and device integrations are organized separately
for future expansion; Anthropic and other displays are not implemented yet.
ChatGPT is the display name, not a claim that the reported quota covers all
ChatGPT features. The dashboard shows the specific quota pool returned by the API.

The current device UI, management page and banner captions are in Chinese.
This English README does not add an English UI.

## Features

- **Quota dashboard:** remaining percentages, progress bars and reset times for
  the 5-hour and 7-day windows. When only the weekly limit applies, it fills the row.
- **Available resets:** remaining reset count and the nearest known expiration,
  with explicit messages when information is missing or incomplete.
- **Four notes:** edit at `http://127.0.0.1:8888`. Each note has four rows of seven
  full-width characters; ASCII letters and digits take half a character cell.
- **Four banner states:** built-in artwork changes with the remaining quota and
  available resets. Images are not sent over USB.
- **Content-driven refresh:** 1404 × 1872 portrait layout with 16 grayscale levels.
  The header shows the date and weekday, not a minute clock. An unchanged screen
  does not refresh just because a minute has passed; date or content changes can
  trigger a refresh.
- **Local companion service:** starts at login and preserves configuration and
  notes across upgrades. Failed data retrieval retains the last data and marks
  its status.

## Previews

These images use fixed sample data and the same rendering code as the device.
They are not account screenshots or photographs of the physical display.

| Weekly only · 2 resets · Sprint | Both windows · No resets · Cruise |
| --- | --- |
| ![Weekly quota at 79%, two resets, sprint banner](docs/assets/dashboard.png) | ![Both windows, 5-hour quota at 50%, no resets, cruise banner](docs/assets/dashboard-cruise.png) |
| **Both windows · No resets · Snack** | **Both windows · No resets · Sleep** |
| ![Both windows, 5-hour quota at 20%, no resets, snack banner](docs/assets/dashboard-snack.png) | ![Both windows, 5-hour quota at 0%, no resets, sleep banner](docs/assets/dashboard-sleep.png) |

Banner selection uses the 5-hour remaining percentage when available, otherwise
the weekly remaining percentage, plus **100 per available reset**:
70 or above → sprint; 30–<70 → cruise; 1–<30 → snack; below 1 → sleep.
Resets affect banner selection only, not the progress bars. Open an image to see
its full resolution.

The bottom-right timestamp records when the Mac generated the display content,
not the latest quota query or an acknowledgment that the screen refreshed.
The web preview represents the home-page content to synchronize, not a screen
capture. USB connection status is shown separately on the management page.

## Installation and upgrades

Current application version: **1.0.0**. Download the complete archive and matching
checksum from [Releases](https://github.com/zzzblxx/glance/releases):

- `glance-macos-arm64.tar.gz`
- `glance-macos-arm64.tar.gz.sha256`

The automatically generated GitHub **Source code** archives are not installation
packages. Detailed [installation instructions](docs/install.md) and
[release notes](docs/release.md) are also available; the installation document is
in Chinese, while the steps below are self-contained in English.

### Requirements

- An Apple Silicon Mac, an E1003 and a USB cable that supports data transfer.
- Codex CLI installed and authenticated with `codex login` under the same macOS
  user who will run Glance.
- A working serial port. If `/dev/cu.wchusbserial…` is missing, follow the device
  and WCH vendor instructions to install and enable the serial driver extension.
- Internet access on the Mac for quota retrieval.

No `sudo` is required. Do not manually delete an existing installation or run
another serial monitor or companion instance during installation.
The binaries are not Developer ID signed or notarized. If macOS blocks them,
verify their source and use the normal system approval flow; do not disable
Gatekeeper globally.

### 1. Verify and extract

Put both downloaded files in Downloads and run in one terminal:

```sh
cd "$HOME/Downloads" &&
shasum -a 256 -c glance-macos-arm64.tar.gz.sha256 &&
glance_release_dir="$(mktemp -d "$HOME/Downloads/glance-install.XXXXXX")" &&
tar -xzf glance-macos-arm64.tar.gz -C "$glance_release_dir" &&
cd "$glance_release_dir" &&
./bin/glance version
```

The checksum must report `OK`. Stop if verification fails. Continue using this
terminal in the extracted directory for the following commands.

Find your device's port:

```sh
./bin/glance-device --list
```

**Replace `/dev/cu.wchusbserial20120` in every command below with your actual port.**

### 2A. Upgrade an existing installation

Version 1.0.0 uses renderer compatibility ID `glance-3`. Upgrading from a
pre-release build requires updating both the Mac service and device firmware:

```sh
export PATH="$HOME/.local/bin:$PATH" &&
bash scripts/stop-service.sh &&
bash scripts/flash-e1003.sh /dev/cu.wchusbserial20120 &&
bash scripts/install.sh
```

Do not disconnect USB while flashing. The script uses 115200 baud, verifies the
four bundled firmware images and writes them without a full-chip erase.
Existing notes, configuration and pairing are preserved; the previous
installation is archived. You do not need another factory backup or login.
If a step fails, stop and keep its complete error output. Installer rollback does
not restore firmware already written to the device.

For a future release that explicitly states the firmware remains compatible,
running `bash scripts/install.sh` may be sufficient. Do not skip the firmware
update when moving from a pre-release build to 1.0.0.

### 2B. Set up a new device

Use this path only for a device that has not yet been flashed with Glance.
Close other serial applications, then identify the chip and flash capacity:

```sh
./bin/esptool --chip esp32s3 --port /dev/cu.wchusbserial20120 flash-id
```

If the detected flash is **32 MB**, save a complete factory backup:

```sh
mkdir -p "$HOME/Library/Application Support/Glance Backups" &&
glance_factory_dir="$(mktemp -d "$HOME/Library/Application Support/Glance Backups/factory.XXXXXX")" &&
./bin/esptool --chip esp32s3 --port /dev/cu.wchusbserial20120 --baud 115200 \
  read-flash 0x0 0x2000000 "$glance_factory_dir/factory-32mb.bin" &&
shasum -a 256 "$glance_factory_dir/factory-32mb.bin"
```

Do not copy this read length for a different flash capacity. A full read can take
about 50 minutes; do not flash until it succeeds. Keep the backup private and
retain it separately from the downloaded installation files.

The following commands **replace the factory firmware**, create the initial
configuration, pair the device and install the service:

```sh
export PATH="$HOME/.local/bin:$PATH" &&
bash scripts/flash-e1003.sh /dev/cu.wchusbserial20120 &&
./bin/glance init-config &&
./bin/glance-device --port /dev/cu.wchusbserial20120 &&
bash scripts/install.sh
```

`init-config` is for first-time setup and will not overwrite an existing file.
Pairing records the port and device identity; it does not transmit account
credentials. The device does not need Wi-Fi configuration.

### 3. Check the installation

Open **http://127.0.0.1:8888**. Check that USB is connected and the preview matches
the device's home page. Edit and save a note to confirm synchronization.
Grayscale full-screen updates can briefly flash black; unchanged content should
not cause a flash every minute.

| Item | Location |
| --- | --- |
| Programs, configuration and data | `~/Library/Application Support/Glance/` |
| Management page | `http://127.0.0.1:8888` |
| Login service | `com.glance.companion` |
| Upgrade and uninstall backups | `~/Library/Application Support/Glance Backups/` |

Check the installed version and service status:

```sh
"$HOME/Library/Application Support/Glance/bin/glance" version
launchctl print "gui/$(id -u)/com.glance.companion"
```

Once installed, Glance no longer depends on the extracted Downloads directory.
There is no need to move it to Documents or Applications. Do not delete factory
backups when cleaning Downloads. Installation backups can contain credentials
and should remain private.

### Maintenance

Notes saved through the management page do not require a restart. After editing
other configuration, restart the service with:

```sh
launchctl kickstart -k "gui/$(id -u)/com.glance.companion"
```

If the port or device changes, run these commands from an extracted complete
package, using the new port:

```sh
bash scripts/stop-service.sh &&
./bin/glance-device --port /dev/cu.wchusbserial20120 &&
bash scripts/install.sh
```

To uninstall, run `bash scripts/uninstall.sh` from the complete package. It stops
the service and archives the application and launch agent, preserving notes,
configuration, pairing and runtime data. It does not remove Codex login data.

If `codex` is not found, open a new terminal or add its installation directory to
`PATH` (`$HOME/.local/bin` for the standalone installer). For connection problems,
check the cable, driver, port and pairing; firmware and host programs must use
compatible renderer versions. Do not run a second service instance to diagnose
a running background service. More troubleshooting is in the
[Chinese installation guide](docs/install.md#常见问题).

## Editing notes

![Four note editors](docs/assets/notes-editor.png)

Each editor has four rows, each seven full-width characters wide: up to 28 Chinese
characters, or more half-width ASCII characters. Blank notes and explicit line
breaks are supported. Automatic wrapping does not alter the saved text; an
explicit line break consumes a row. Notes that exceed the layout capacity cannot
be saved. You can edit while the device is disconnected; it synchronizes after
reconnecting.

## Architecture and limitations

Go handles collection, normalization and the management service. USB carries
structured JSON only; the ESP32-S3 renders with C++. The Mac preview shares the
device's drawing code and bundled font. Quota data currently comes from OpenAI's
Codex app-server, not a multi-provider billing or cost API. Provider identity,
display name and data access are separate, and the heading comes from the data.

- Task details require verifiable local activity records. **Remote Codex task
  activity is not collected.** Missing activity does not imply a quota or USB fault.
- The white hardware button changes pages. Touch input, partial refresh and a
  live minute clock are not implemented.
- The bundled Unifont bitmap font does not support non-BMP characters, including
  most emoji.
- Glance does not prevent Mac sleep. A sleeping Mac or disconnected device leaves
  the old screen visible; that does not mean its data is current.
- The management page is local-only, not exposed to the LAN or internet. USB is
  treated as a trusted physical connection.

## Development documentation

Detailed development guides are currently in Chinese. Release notes are bilingual.

| Document | Contents |
| --- | --- |
| [Installation](docs/install.md) | Setup, upgrades, pairing, troubleshooting and uninstall |
| [Build and release](docs/build.md) | Tool versions, builds, tests and packaging |
| [Architecture](docs/architecture.md) | Provider, device, transport and layout boundaries |
| [Configuration and data safety](docs/configuration.md) | Settings, storage and security boundaries |
| [Display styling](docs/styling.md) | Fonts, notes, banners and refresh rules |
| [USB protocol](docs/protocol.md) | Data model, handshake, validation and transport constraints |
| [Acceptance checklist](docs/acceptance.md) | Automated and hardware checks |

```text
cmd/                              Go entry points and asset generation tools
internal/providers/openai/codex/   OpenAI data access through Codex
internal/devices/e1003/            E1003 profile and native preview adapter
internal/transport/serial/         Device-independent serial transport
internal/protocol/                 Display protocol and page data projection
internal/                         Scheduling, models, config, server, storage and tests
firmware/e1003/                    E1003 firmware, layout and build configuration
firmware/shared/                   Shared generated assets
assets/fonts/                     Font assets and licenses
assets/banners/                   Original banner images
native/e1003/                     Mac preview helper and native tests
scripts/                          Build, installation, packaging and validation
artifacts/e1003/                   Flashable firmware and checksums
docs/                             Guides, previews and third-party notices
VERSION                           Application version
```

`bin/`, `dist/`, `var/` and tool caches are local outputs, not tracked in Git.
Follow the [build guide](docs/build.md) to prepare dependencies before running
`make test`.

## License

Glance's own code, documentation and all four banner images are licensed under
the [MIT License](LICENSE), Copyright (c) 2026 zzzblxx. Use, modification,
redistribution and commercial use are permitted subject to retaining the
copyright and permission notice. This covers the original images and generated
embedded data; see the [banner documentation](assets/banners/README.md).

Third-party dependencies and fonts retain their own licenses. The project's MIT
license does not replace them; see the [third-party notices](docs/third-party.md).
