# Distribution licenses

Hermit's own code remains MIT-licensed under the root LICENSE. The application includes Mihomo and is distributed as a GPLv3 combined work. Preserve both notices when distributing it.

`mihomo-GPL-3.0.txt` is copied verbatim from Mihomo v1.19.3. Release packaging collects the other Go and production npm dependency notices into `licenses/go/` and `licenses/npm/` in the app archive, and includes matching dependency sources in the source archive. Missing notices stop packaging.

The source archive contains the tagged Hermit source, Go vendor sources, production npm packages, Wails runtime TypeScript source, and the matching React upstream source archive. Build tools and development dependencies are installed at the versions pinned in go.mod and the npm lockfile; see docs/releasing.md for build instructions.
