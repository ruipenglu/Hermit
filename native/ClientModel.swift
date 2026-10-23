import AppKit
import Foundation
import SwiftUI
import Combine

struct Mapping: Decodable, Identifiable, Sendable {
    let id: String
    let port: Int
    let source: String
    let target: String?
    let enabled: Bool
    let hasCredential: Bool
}

struct TestRecord: Decodable, Identifiable, Sendable {
    let time: String
    let port: Int
    let expectedProxy: String
    let actualChain: [String]
    let exitIp: String?
    let latencyMs: Int?
    let status: String
    let identityConfirmed: Bool
    let message: String
    var id: String { time + String(port) }
    var date: String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: time)?.formatted(date: .abbreviated, time: .shortened) ?? time
    }
}

struct CoreState: Decodable, Sendable {
    let enabled: Bool
    let revision: Int
    let appliedRevision: Int?
    let mappings: [Mapping]
    let history: [TestRecord]
    let recoveryRequired: Bool
    let status: String
    let errors: [String]
    static let empty = CoreState(enabled: false, revision: 0, appliedRevision: nil, mappings: [], history: [],
                                 recoveryRequired: false, status: "loading", errors: [])
    var pending: Bool { enabled && revision != appliedRevision }
}

struct Node: Decodable, Identifiable, Sendable {
    let name: String
    let type: String
    var id: String { name }
}

struct EnvironmentInfo: Decodable, Sendable {
    let clashVersion: String
    let mihomoVersion: String
    let atomicNodeCount: Int
    let dataDirectory: String
    let script: String
}

struct MappingInput: Encodable, Sendable {
    var id: String?
    var port: Int?
    var source: String?
    var enabled: Bool?
    var target: String?
    var uri: String?
}

struct CoreRequest: Encodable, Sendable {
    let action: String
    var data: MappingInput?
}

struct CoreReply<Value: Decodable>: Decodable {
    let ok: Bool
    let result: Value?
    let error: String?
}

struct EmptyReply: Decodable {}

struct ClientError: LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

// One-shot helpers keep credentials in private pipes and require no local control server.
func runCore(_ data: Data, executable: URL) async throws -> Data {
    try await withCheckedThrowingContinuation { continuation in
        DispatchQueue.global(qos: .userInitiated).async {
            let process = Process()
            let input = Pipe(), output = Pipe()
            process.executableURL = executable
            process.arguments = ["--request"]
            process.standardInput = input
            process.standardOutput = output
            process.standardError = FileHandle.nullDevice
            do {
                try process.run()
                try input.fileHandleForWriting.write(contentsOf: data)
                try input.fileHandleForWriting.close()
                let reply = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                guard !reply.isEmpty else {
                    throw ClientError(message: "核心进程未返回结果，请检查应用是否完整；如有未完成事务，请先恢复。")
                }
                continuation.resume(returning: reply)
            } catch {
                try? input.fileHandleForWriting.close()
                if process.isRunning { process.terminate(); process.waitUntilExit() }
                continuation.resume(throwing: ClientError(message: "无法完成核心操作，请检查应用是否完整及本地权限。"))
            }
        }
    }
}

enum Page: String, CaseIterable, Identifiable {
    case mappings = "端口映射", nodes = "活动节点", tests = "测试记录", environment = "环境与恢复"
    var id: String { rawValue }
    var icon: String {
        switch self {
        case .mappings: "point.3.connected.trianglepath.dotted"
        case .nodes: "server.rack"
        case .tests: "waveform.path.ecg"
        case .environment: "gearshape"
        }
    }
}

@MainActor
final class MappingDraft: ObservableObject, Identifiable {
    let id = UUID()
    let mapping: Mapping?
    @Published var port: String
    @Published var source: String
    @Published var target: String
    @Published var enabled: Bool
    @Published var uri = ""
    @Published var showURI = false

    init(mapping: Mapping? = nil, port: Int = 18080, target: String = "") {
        self.mapping = mapping
        self.port = String(mapping?.port ?? port)
        self.source = mapping?.source ?? "active-profile"
        self.target = mapping?.target ?? target
        self.enabled = mapping?.enabled ?? true
    }
}

@MainActor
final class ClientModel: ObservableObject {
    @Published var state = CoreState.empty
    @Published var nodes: [Node] = []
    @Published var environment: EnvironmentInfo?
    @Published var environmentError: String?
    @Published var nodesError: String?
    @Published var page: Page? = .mappings
    @Published var editor: MappingDraft?
    @Published var busy = false
    @Published var activity = ""
    @Published var notice: String?
    @Published var error: String?
    @Published var terminationRequested = false

    let executable = Bundle.main.resourceURL!.appendingPathComponent("core/hermit-core")

    func call<Value: Decodable>(_ action: String, data: MappingInput? = nil, as type: Value.Type) async throws -> Value {
        let request = try JSONEncoder().encode(CoreRequest(action: action, data: data))
        let raw = try await runCore(request, executable: executable)
        let reply: CoreReply<Value>
        do { reply = try JSONDecoder().decode(CoreReply<Value>.self, from: raw) }
        catch { throw ClientError(message: "核心返回的数据格式无效，请重新安装完整应用。") }
        guard reply.ok, let value = reply.result else {
            throw ClientError(message: reply.error ?? "核心操作未完成")
        }
        return value
    }

    func finish() {
        busy = false
        activity = ""
        if terminationRequested { NSApp.reply(toApplicationShouldTerminate: true) }
    }

    func refresh(all: Bool = true) async {
        guard !busy, !terminationRequested else { return }
        busy = true
        defer { finish() }
        do { state = try await call("status", as: CoreState.self) }
        catch { self.error = error.localizedDescription }
        if all {
            do { nodes = try await call("nodes", as: [Node].self); nodesError = nil }
            catch { nodes = []; nodesError = error.localizedDescription }
            do { environment = try await call("doctor", as: EnvironmentInfo.self); environmentError = nil }
            catch { environment = nil; environmentError = error.localizedDescription }
        }
    }

    @discardableResult
    func perform(_ action: String, data: MappingInput? = nil) async -> Bool {
        guard !busy, !terminationRequested else { return false }
        busy = true
        activity = action == "test" ? "正在验证出口和代理链…" : "正在处理，请稍候…"
        defer { finish() }
        do {
            if action == "test" {
                let result = try await call(action, data: data, as: TestRecord.self)
                notice = result.message
                page = .tests
            } else {
                let _: EmptyReply = try await call(action, data: data, as: EmptyReply.self)
                switch action {
                case "save", "delete": notice = "映射已保存。开启总开关或应用更改后生效。"
                case "disable": notice = "已关闭固定端口并恢复原脚本。"
                case "recover": notice = "已恢复到变更前状态。"
                default: notice = "固定端口已应用。可点击测试确认指定出口。"
                }
            }
            state = try await call("status", as: CoreState.self)
            return true
        } catch {
            self.error = error.localizedDescription
            if let current = try? await call("status", as: CoreState.self) { state = current }
            return false
        }
    }

    func newMapping(target: String = "") {
        var port = 18080
        while state.mappings.contains(where: { $0.port == port }) { port += 1 }
        editor = MappingDraft(port: port, target: target)
    }
    func canTest(_ mapping: Mapping) -> Bool {
        state.enabled && mapping.enabled && !state.pending && !state.recoveryRequired && !busy
    }

    func mappingStatus(_ mapping: Mapping) -> String {
        if !mapping.enabled { return "已停用" }
        if !state.enabled { return "未启用" }
        if state.pending { return "待应用" }
        return state.status == "applied" ? "已应用" : "需检查"
    }

    var statusTitle: String {
        switch state.status {
        case "loading": "正在读取状态"
        case "disabled": "固定端口尚未启用"
        case "applied": "固定端口已应用"
        case "saved-pending-apply": "映射已保存，等待应用"
        case "degraded": "运行状态需要检查"
        case "unavailable": "Clash 暂不可用"
        case "conflict": "全局脚本存在冲突"
        case "recovery-required": "上次操作尚未完成"
        default: "请检查当前状态"
        }
    }

    var statusDetail: String {
        if state.recoveryRequired { return "请恢复变更前状态，再继续操作。" }
        if state.pending { return "当前运行的仍是上次应用的配置。" }
        if state.enabled { return "监听检查仅证明入口可用，指定出口身份需通过连接测试确认。" }
        return "每个本地端口连接一个指定节点，现有分流保持不变。"
    }
}
