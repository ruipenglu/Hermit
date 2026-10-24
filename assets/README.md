# Hermit Logo

采用尖顶海螺候选图中的 01：深蓝壳体、青色上层与内螺旋，透明留白。

- `Hermit.png`：透明背景原始 Logo，1254 × 1254。
- `Hermit.icns`：macOS 图标，与 `build/darwin/icons.icns` 同步。
- `../build/appicon.png`：1024 × 1024 打包输入。
- `../build/appicon.icon/`：Icon Composer 源文件，生成 `Assets.car` 和 macOS 图标。
- `../frontend/public/hermit.png`：128 × 128 顶栏及浏览器图标。

Logo 通过内置 imagegen 从用户选定的候选 01 提取。生成提示词：

> Extract and faithfully reproduce ONLY logo 01, top-left icon of the reference contact sheet, as ONE standalone high resolution logo on genuine transparent background. Preserve the EXACT selected design: broad round navy shell belly, white/transparent negative-space curled spiral at lower left containing teal curl, upper right tapered conical spire composed of a broad teal band and small navy triangular tip, two clean transparent curved seams. Same lean, silhouette, proportions, colors, and geometry as reference 01. Do not redesign or reinterpret. Remove all other logos, number labels, and white backdrop. Flat solid navy and teal fills, crisp smooth vector-like edges without texture or gradients. Center complete mark in a square canvas with approximately 12 percent transparent margin on each side, no cropping, no extra objects, no text, no shadow. Production logo asset.
