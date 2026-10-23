import Foundation

@main
struct NativeSmoke {
    @MainActor
    static func main() async throws {
        guard CommandLine.arguments.count == 2 else { fatalError("Supply the packaged core path") }
        let core = URL(fileURLWithPath: CommandLine.arguments[1])
        let encoder = JSONEncoder(), decoder = JSONDecoder()
        let raw = try await runCore(encoder.encode(CoreRequest(action: "status")), executable: core)
        let reply = try decoder.decode(CoreReply<CoreState>.self, from: raw)
        guard reply.ok, let state = reply.result else { throw ClientError(message: reply.error ?? "No status") }
        assert(state.status != "loading")
        assert(!String(decoding: raw, as: UTF8.self).contains("\"credential\""))

        let invalid = try await runCore(encoder.encode(CoreRequest(action: "unsupported-operation")), executable: core)
        let failure = try decoder.decode(CoreReply<EmptyReply>.self, from: invalid)
        assert(!failure.ok && failure.error != nil)

        let model = ClientModel()
        model.state = state
        model.newMapping(target: "test-node")
        assert(model.editor?.target == "test-node")
        assert(!state.mappings.contains { String($0.port) == model.editor?.port })
        assert(model.editor?.uri.isEmpty == true)
        model.busy = true
        let busyRequest = await model.perform("enable")
        assert(!busyRequest)
        model.busy = false
        model.terminationRequested = true
        let terminatingRequest = await model.perform("enable")
        assert(!terminatingRequest)
        print("Native Swift stdio, DTO decoding, safe errors, editor defaults and operation guards passed.")
    }
}
