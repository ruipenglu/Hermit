import AppKit
import SwiftUI

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    weak var model: ClientModel?

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard let model, model.busy else { return .terminateNow }
        // A configuration transaction must finish before its owning client exits.
        model.terminationRequested = true
        return .terminateLater
    }
}

@main
struct HermitApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @StateObject private var model = ClientModel()

    var body: some Scene {
        Window("Hermit", id: "main") {
            ContentView(model: model)
                .onAppear { delegate.model = model }
                .frame(minWidth: 920, minHeight: 640)
        }
        .defaultSize(width: 1120, height: 760)
        .windowResizability(.contentMinSize)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("新增映射") { model.page = .mappings; model.newMapping() }
                    .keyboardShortcut("n").disabled(model.busy)
            }
            CommandGroup(after: .newItem) {
                Button("刷新状态") { Task { await model.refresh() } }
                    .keyboardShortcut("r").disabled(model.busy)
                Button("应用更改") { Task { await model.perform("apply") } }
                    .keyboardShortcut(.return, modifiers: .command)
                    .disabled(model.busy || !model.state.pending || model.state.recoveryRequired)
            }
        }
    }
}
