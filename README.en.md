<p align="center">
  <img src="assets/Hermit.png" width="112" alt="Hermit shell logo" />
</p>
<h1 align="center">Hermit</h1>
<p align="center">A dedicated local proxy port for each app.</p>
<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="#get-started">Get started</a> ·
  <a href="docs/troubleshooting.md">Troubleshooting</a> ·
  <a href="https://github.com/ruipenglu/Hermit/issues">Feedback</a>
</p>

Hermit is a local proxy port manager for Clash Verge Rev on macOS. Bind a port to a specific node, then use that address in your app. Give browsers, command-line tools, and test environments their own proxy routes while keeping your existing Clash routing rules.

![Hermit: manage port mappings, inspect node status, and copy proxy addresses](docs/images/preview.png)

<p align="center"><sub>The screenshot uses fictional nodes and example IP addresses.</sub></p>

## One port, one chosen node

Give each app its own proxy address without repeatedly switching a shared proxy entry point. For example:

| Use case | Local proxy address | Assigned node |
| --- | --- | --- |
| Browser | `127.0.0.1:18080` | Node A |
| Command-line tools | `127.0.0.1:18081` | Node B |
| Test environment | `127.0.0.1:18082` | Node C |

Each app needs to support manual HTTP or SOCKS5 proxy settings. Hermit manages port mappings; Clash Verge Rev manages your nodes and subscriptions.

## Everyday controls in one window

- **Assign a node**: choose an active node or import a node URI, then give it a local port.
- **Copy an address**: click a port, choose HTTP or SOCKS5, and paste the URL into your app.
- **Check the route**: test a connection inline to see its exit IP, latency, and whether the actual proxy chain includes your chosen node.
- **Control all mappings**: edits apply automatically while enabled. Switching off restores the original script and keeps your mappings for later.

Search by port or node, use light or dark appearance, and work comfortably in a small window.

## Get started

The current build targets **Apple Silicon Macs with a macOS 13 minimum**. Start Clash Verge Rev with a working configuration before using Hermit. A downloadable app has not been released yet; to try it now, follow the [source build guide](CONTRIBUTING.md#构建-macos-应用).

1. Open Hermit and add a mapping.
2. Enter a local port, choose a node, and save.
3. Enable the mapping master switch, then click the port to copy its proxy address.
4. Paste the address into your app's proxy settings and test the connection in Hermit.

Saving while the master switch is off only stores the mapping. **Save and test turns on the master switch and applies all enabled mappings.**

Supported node URI families: VLESS, VMess, Trojan, Shadowsocks, ShadowsocksR, Hysteria, Hysteria2, and TUIC.

## Data and privacy

Node credentials and recovery snapshots are encrypted locally, with the key stored in macOS Keychain. Hermit has no built-in telemetry. Connection tests reach `api.ipify.org` through the selected proxy to retrieve the exit IP.

Enabling mappings changes Clash's global script and reloads its configuration; switching off restores the original script. Disable mappings before upgrading or uninstalling. See [security and privacy](SECURITY.md) for details and reporting channels, or [troubleshooting](docs/troubleshooting.md) for help.

## Feedback and contributions

Report problems and suggest improvements through [Issues](https://github.com/ruipenglu/Hermit/issues). Include your version, reproduction steps, and redacted screenshots. Never attach real subscriptions, credentials, or full configurations.

[Changelog](CHANGELOG.md) · [Contributing](CONTRIBUTING.md) · [Compatibility and validation](PLAN.md#未验收与已知限制)

Hermit's own code is licensed under [MIT](LICENSE). Distributing the app with Mihomo also requires compliance with GPLv3; see [third-party notices](THIRD_PARTY_NOTICES.md).
