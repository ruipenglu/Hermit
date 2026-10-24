<p align="center"><img src="assets/Hermit.png" width="112" alt="Hermit shell logo" /></p>

# Hermit

**Bind a local proxy port to a specific node without replacing your existing routing rules.**

Hermit is a macOS companion for Clash Verge Rev. It is useful when different applications or development tools need separate, predictable proxy exits.

[简体中文](README.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)

> **Early development; project license undecided.** This repository is not yet a fully licensed open-source distribution. It directly includes GPLv3-covered Mihomo code. Project licensing and dependency obligations must be resolved before redistribution; see [third-party notices](THIRD_PARTY_NOTICES.md). No Developer ID signed or notarized release is currently provided.

![Hermit with fictional nodes and documentation-only IP addresses](docs/images/preview.png)

## Features

- Map local HTTP / SOCKS5 ports to active Clash nodes or imported node URIs.
- Apply edits while enabled; restore the original script when disabled and retain mappings.
- Verify exit IP, latency, and the actual proxy chain, not just connectivity.
- Search mappings, copy proxy URLs, inspect inline results, and use light or dark appearance.
- Encrypt credentials and recovery snapshots using a macOS Keychain key; recover interrupted transactions.

Hermit requires Clash Verge Rev. It does not supply proxy servers, replace subscription management, or run an independent proxy kernel.

## Requirements and build

The current app target is Apple Silicon macOS, with a macOS 13 build minimum. Not every OS version has been tested. Read-only integration was checked with Clash Verge Rev 2.5.8 and Mihomo v1.19.32. Intel, other desktop platforms, full real-environment mutation tests, and Apple notarization remain unverified.

Install Go 1.27.3, Node.js 24, npm, and Xcode Command Line Tools. Full Xcode is needed to regenerate Icon Composer assets.

```sh
git clone https://github.com/ruipenglu/Hermit.git
cd Hermit
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 package
open bin/hermit.app
```

The build installs locked dependencies and signs locally. Ad-hoc signing is not notarization. Do not disable system-wide security controls to run the app.

## Use

1. Start Clash Verge Rev with a valid configuration.
2. Add a local port (1024–65535) and choose an active node or paste a supported node URI.
3. Enable the mapping master switch. Saving while disabled only stores configuration. **Save and test enables the master switch and applies every enabled mapping.**
4. Configure the target application with `127.0.0.1` and the chosen port. The port's copy menu offers HTTP and SOCKS5 URLs.
5. Test the connection and inspect node verification, exit IP and proxy chain.

Supported URI families: VLESS, VMess, Trojan, Shadowsocks, ShadowsocksR, Hysteria, Hysteria2 and TUIC, as parsed by the bundled Mihomo converter.

## Privacy and development

Data is stored in `~/Library/Application Support/Hermit/`. Credentials and recovery snapshots are encrypted; their key is in Keychain. Node names, port numbers and recent test summaries can still be sensitive.

Clash control uses a local Unix socket. Enabling mappings changes Clash's global script and reloads configuration. A connection test reaches `https://api.ipify.org` through the selected proxy. There is no built-in telemetry or automatic updater. OS integration invokes standard macOS tools and the existing Mihomo configuration validator.

```sh
cd frontend
npm ci
npm run dev       # isolated browser preview at http://127.0.0.1:9245
npm run typecheck
npm test
npm run build
# return to repository root:
cd ..
go test -race ./internal/...
go vet ./...
```

Only development browser sessions use mock data. Production builds and `wails3 dev` use the real backend. Do not delete application data while mappings are enabled; turn them off and preserve recovery data first.

See [contributing](CONTRIBUTING.md), [architecture](docs/architecture.md), [troubleshooting](docs/troubleshooting.md), and [release preparation](docs/releasing.md). `PLAN.md` is the source of truth for implementation status and validation boundaries. Never attach real subscription URLs, credentials, state files or full Clash configurations to a public issue.
