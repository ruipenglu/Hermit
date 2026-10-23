import SwiftUI
import Combine

struct ContentView: View {
    @ObservedObject var model: ClientModel
    private let timer = Timer.publish(every: 10, on: .main, in: .common).autoconnect()

    var body: some View {
        NavigationSplitView {
            List(Page.allCases, selection: $model.page) { page in
                Label(page.rawValue, systemImage: page.icon).tag(page)
                    .padding(.vertical, 6).accessibilityIdentifier("nav-" + page.id)
            }
            .navigationTitle("Hermit")
            .navigationSplitViewColumnWidth(min: 180, ideal: 200, max: 260)
            .safeAreaInset(edge: .bottom) {
                Label(model.environment == nil ? "Clash 环境需检查" : "已连接 Clash Verge Rev",
                      systemImage: model.environment == nil ? "circle" : "checkmark.circle.fill")
                    .font(.caption).foregroundStyle(.secondary).padding()
            }
        } detail: {
            VStack(spacing: 0) {
                statusHeader
                Divider()
                if !model.state.errors.isEmpty {
                    Text(model.state.errors.joined(separator: "；"))
                        .font(.callout).foregroundStyle(.orange).frame(maxWidth: .infinity, alignment: .leading)
                        .padding().background(.orange.opacity(0.07))
                }
                switch model.page ?? .mappings {
                case .mappings: mappings
                case .nodes: nodes
                case .tests: history
                case .environment: environment
                }
                Divider()
                HStack(spacing: 8) {
                    if !model.activity.isEmpty { ProgressView().controlSize(.small) }
                    Text(model.terminationRequested ? "正在完成当前操作，随后退出…" :
                         !model.activity.isEmpty ? model.activity : model.notice ?? "本地优先 · v1.0.0")
                        .font(.caption).foregroundStyle(.secondary).lineLimit(2)
                    Spacer()
                    Text("127.0.0.1 · HTTP / SOCKS5").font(.caption).foregroundStyle(.tertiary)
                }.padding(.horizontal, 20).padding(.vertical, 10)
            }
            .navigationTitle((model.page ?? .mappings).rawValue)
            .toolbar {
                Button { Task { await model.refresh() } } label: { Label("刷新", systemImage: "arrow.clockwise") }
                    .disabled(model.busy).accessibilityIdentifier("refresh")
            }
        }
        .task { await model.refresh() }
        .onReceive(timer) { _ in Task { await model.refresh(all: false) } }
        .onReceive(NotificationCenter.default.publisher(for: NSApplication.didBecomeActiveNotification)) { _ in
            Task { await model.refresh() }
        }
        .sheet(item: $model.editor) { draft in MappingEditor(model: model, draft: draft) }
        .alert("操作失败", isPresented: Binding(get: { model.error != nil && model.editor == nil }, set: { if !$0 { model.error = nil } })) {
            Button("好") { model.error = nil }
        } message: { Text(model.error ?? "") }
    }

    private var statusHeader: some View {
        HStack(spacing: 16) {
            Image(systemName: model.state.enabled ? "network.badge.shield.half.filled" : "network")
                .font(.system(size: 30)).foregroundStyle(model.state.enabled ? Color.accentColor : .secondary)
                .frame(width: 48, height: 48).background(.quaternary, in: RoundedRectangle(cornerRadius: 12))
            VStack(alignment: .leading, spacing: 7) {
                Text(model.statusTitle).font(.title3.weight(.semibold))
                Text(model.statusDetail).font(.caption).foregroundStyle(.secondary)
            }
            Spacer(minLength: 10)
            if model.state.recoveryRequired {
                Button("恢复未完成操作") { Task { await model.perform("recover") } }
                    .disabled(model.busy).accessibilityIdentifier("recover")
            } else {
                if model.state.pending {
                    Button("应用更改") { Task { await model.perform("apply") } }
                        .disabled(model.busy).accessibilityIdentifier("apply")
                }
                Toggle("启用固定端口", isOn: Binding(get: { model.state.enabled }, set: { enabled in
                    Task { await model.perform(enabled ? "enable" : "disable") }
                }))
                .labelsHidden().toggleStyle(.switch)
                .disabled(model.busy || model.state.status == "loading" || model.state.status == "conflict")
                .accessibilityLabel("启用固定端口").accessibilityIdentifier("power")
            }
        }.padding(24)
    }

    private var mappings: some View {
        VStack(spacing: 0) {
            HStack {
                VStack(alignment: .leading, spacing: 5) {
                    Text("固定出口").font(.headline)
                    Text("将独立端口绑定到明确节点").font(.caption).foregroundStyle(.secondary)
                }
                Spacer()
                Button { model.newMapping() } label: { Label("新增映射", systemImage: "plus") }
                    .buttonStyle(.borderedProminent).disabled(model.busy).accessibilityIdentifier("add-mapping")
            }.padding(20)
            if model.state.mappings.isEmpty {
                ContentUnavailableView("从一个固定出口开始", systemImage: "point.3.connected.trianglepath.dotted",
                                       description: Text("添加端口，选择活动节点或输入 VLESS URI。"))
            } else {
                Table(model.state.mappings.sorted { $0.port < $1.port }) {
                    TableColumn("本地端口") { mapping in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(String(mapping.port)).font(.system(.body, design: .monospaced).weight(.semibold))
                            Text("127.0.0.1").font(.caption2).foregroundStyle(.secondary)
                        }.padding(.vertical, 8)
                    }.width(min: 95, ideal: 110)
                    TableColumn("目标节点") { mapping in
                        VStack(alignment: .leading, spacing: 4) {
                            Text(mapping.source == "manual-uri" ? "手动 VLESS 节点" : mapping.target ?? "未知节点")
                                .lineLimit(2).textSelection(.enabled)
                            Text(mapping.source == "manual-uri" ? "凭据加密保存在本机" : "当前活动配置")
                                .font(.caption2).foregroundStyle(.secondary)
                        }
                    }
                    TableColumn("状态") { mapping in
                        Text(model.mappingStatus(mapping)).font(.caption)
                            .foregroundStyle(model.mappingStatus(mapping) == "已应用" ? Color.green : .secondary)
                    }.width(80)
                    TableColumn("操作") { mapping in
                        HStack {
                            Button("测试") { Task { await model.perform("test", data: MappingInput(id: mapping.id)) } }
                                .disabled(!model.canTest(mapping)).accessibilityIdentifier("test-" + String(mapping.port))
                            Button("编辑") { model.editor = MappingDraft(mapping: mapping) }
                                .disabled(model.busy).accessibilityIdentifier("edit-" + String(mapping.port))
                        }
                    }.width(125)
                }.tableStyle(.inset(alternatesRowBackgrounds: true))
            }
            HStack { Text("\(model.state.mappings.count) 条映射"); Spacer(); Text("保存后需应用更改；首次使用请开启总开关。") }
                .font(.caption).foregroundStyle(.secondary).padding(16)
        }
    }

    private var nodes: some View {
        List {
            if let error = model.nodesError { Text(error).foregroundStyle(.orange) }
            if model.nodes.isEmpty { Text("没有可用的原子代理节点，请检查 Clash 配置。").foregroundStyle(.secondary) }
            ForEach(model.nodes) { node in
                HStack(spacing: 14) {
                    Image(systemName: "server.rack").foregroundStyle(.tint).frame(width: 28)
                    VStack(alignment: .leading, spacing: 5) {
                        Text(node.name).textSelection(.enabled)
                        Text(node.type).font(.caption).foregroundStyle(.secondary)
                    }
                    Spacer()
                    Button("绑定端口") { model.newMapping(target: node.name) }.disabled(model.busy)
                }.padding(.vertical, 8)
            }
        }.safeAreaInset(edge: .bottom) {
            Text("仅列出运行中的原子代理节点，策略组与 DIRECT 不参与绑定。")
                .font(.caption).foregroundStyle(.secondary).padding()
        }
    }

    private var history: some View {
        Group {
            if model.state.history.isEmpty {
                ContentUnavailableView("还没有测试记录", systemImage: "waveform.path.ecg",
                                       description: Text("应用映射后，点击测试确认出口 IP 和指定节点身份。"))
            } else {
                List(model.state.history.reversed()) { record in
                    DisclosureGroup {
                        VStack(alignment: .leading, spacing: 8) {
                            Text(record.message)
                            Text("预期节点：" + model.proxyLabel(record.expectedProxy))
                            Text("实际代理链：" + (record.actualChain.isEmpty ? "未捕获" : record.actualChain.map(model.proxyLabel).joined(separator: " → ")))
                        }.font(.callout).foregroundStyle(.secondary).textSelection(.enabled).padding(.vertical, 8)
                    } label: {
                        HStack {
                            Image(systemName: record.identityConfirmed && record.status == "verified" ? "checkmark.circle.fill" : "exclamationmark.circle")
                                .foregroundStyle(record.status == "verified" ? Color.green : .orange)
                            VStack(alignment: .leading, spacing: 4) {
                                Text("127.0.0.1:\(String(record.port))").font(.system(.body, design: .monospaced))
                                Text(record.date).font(.caption).foregroundStyle(.secondary)
                            }
                            Spacer()
                            VStack(alignment: .trailing, spacing: 4) {
                                Text(testLabel(record.status)).foregroundStyle(record.status == "verified" ? Color.green : .orange)
                                Text("\(record.exitIp ?? "未取得出口 IP") · \(record.latencyMs.map(String.init) ?? "—") ms")
                                    .font(.caption).foregroundStyle(.secondary)
                            }
                        }.padding(.vertical, 6)
                    }
                }
            }
        }
    }

    private var environment: some View {
        Form {
            Section("当前环境") {
                if let info = model.environment {
                    LabeledContent("Clash Verge Rev", value: info.clashVersion)
                    LabeledContent("Mihomo", value: info.mihomoVersion)
                    LabeledContent("活动节点", value: "\(info.atomicNodeCount) 个")
                    LabeledContent("Clash 数据目录") { Text(info.dataDirectory).textSelection(.enabled) }
                    LabeledContent("全局脚本") { Text(info.script).textSelection(.enabled) }
                } else { Text(model.environmentError ?? "正在检查环境…").foregroundStyle(.secondary) }
                Button("重新检查环境") { Task { await model.refresh() } }.disabled(model.busy)
            }
            Section("隐私与恢复") {
                Text("手动 URI 和恢复快照加密保存在本机，主密钥位于 macOS 钥匙串。供 Clash 执行的脚本包含内核所需凭据，仅当前用户可读取。")
                Text("检测到外部修改时暂停写入。出现未完成操作时，使用顶部恢复按钮；请保留 Hermit 数据目录和对应钥匙串。")
                Button("在 Finder 中显示数据目录") {
                    NSWorkspace.shared.open(FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/Hermit"))
                }
            }
            Section("连接与退出") {
                Text("测试请求经指定 SOCKS5 端口访问 api.ipify.org，取得出口 IP 并核对该次连接的 Mihomo 代理链；只有身份已确认才代表完整验证成功。")
                Text("退出应用后，已应用的端口继续由 Clash 提供服务。需要停用时请先关闭总开关。")
            }
        }.formStyle(.grouped)
    }

    private func testLabel(_ status: String) -> String {
        switch status {
        case "verified": "身份已确认"
        case "mismatch": "节点不匹配"
        case "unconfirmed": "身份未确认"
        default: "连接失败"
        }
    }
}

struct MappingEditor: View {
    @ObservedObject var model: ClientModel
    @ObservedObject var draft: MappingDraft
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        VStack(spacing: 0) {
            HStack { Text(draft.mapping == nil ? "新增端口映射" : "编辑端口映射").font(.title2.weight(.semibold)); Spacer() }.padding(24)
            Form {
                Section {
                    TextField("本地端口", text: $draft.port).accessibilityIdentifier("mapping-port")
                    Text("监听 127.0.0.1，允许 1024–65535。HTTP 与 SOCKS5 共用此端口。")
                        .font(.caption).foregroundStyle(.secondary)
                }
                Section("节点来源") {
                    Picker("节点来源", selection: $draft.source) {
                        Text("当前活动节点").tag("active-profile")
                        Text("手动 VLESS URI").tag("manual-uri")
                    }.pickerStyle(.segmented).labelsHidden().accessibilityIdentifier("mapping-source")
                    if draft.source == "active-profile" {
                        Picker("目标节点", selection: $draft.target) {
                            Text("请选择节点").tag("")
                            ForEach(model.nodes) { node in Text(node.name + " · " + node.type).tag(node.name) }
                            if !draft.target.isEmpty && !model.nodes.contains(where: { $0.name == draft.target }) {
                                Text(draft.target + "（已失效）").tag(draft.target)
                            }
                        }.accessibilityIdentifier("mapping-target")
                    } else {
                        Group {
                            if draft.showURI { TextField(uriPlaceholder, text: $draft.uri) }
                            else { SecureField(uriPlaceholder, text: $draft.uri) }
                        }.accessibilityIdentifier("mapping-uri")
                        Toggle("显示输入内容", isOn: $draft.showURI).font(.caption)
                        Text("支持 TCP / TLS / REALITY / WebSocket / gRPC。未知参数会明确报错。")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    Toggle("启用此映射", isOn: $draft.enabled).accessibilityIdentifier("mapping-enabled")
                }
            }.formStyle(.grouped).disabled(model.busy)
            Divider()
            HStack {
                if let mapping = draft.mapping {
                    Button("删除映射", role: .destructive) {
                        Task { if await model.perform("delete", data: MappingInput(id: mapping.id)) { draft.uri = ""; dismiss() } }
                    }.accessibilityIdentifier("delete-mapping")
                }
                Spacer()
                Button("取消", role: .cancel) { draft.uri = ""; dismiss() }.keyboardShortcut(.cancelAction)
                Button("保存映射") {
                    guard let value = Int(draft.port) else { model.error = "监听端口必须是 1024–65535 的整数"; return }
                    let input = MappingInput(id: draft.mapping?.id, port: value, source: draft.source, enabled: draft.enabled,
                                             target: draft.source == "active-profile" ? draft.target : nil,
                                             uri: draft.source == "manual-uri" && !draft.uri.isEmpty ? draft.uri : nil)
                    Task { if await model.perform("save", data: input) { draft.uri = ""; dismiss() } }
                }.keyboardShortcut(.defaultAction).buttonStyle(.borderedProminent).accessibilityIdentifier("save-mapping")
            }.padding(20).disabled(model.busy)
        }
        .frame(width: 520, height: 470)
        .interactiveDismissDisabled(model.busy)
        .onDisappear { draft.uri = "" }
        .alert("无法保存映射", isPresented: Binding(get: { model.error != nil }, set: { if !$0 { model.error = nil } })) {
            Button("好") { model.error = nil }
        } message: { Text(model.error ?? "") }
    }

    private var uriPlaceholder: String {
        draft.mapping?.hasCredential == true ? "凭据已保存，留空保留原节点" : "vless://…"
    }
}
