import AppKit

// This executable only opens OS-owned sharing UI. It has no recipient/send API.
func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data(message.utf8))
    exit(1)
}
let input = FileHandle.standardInput.readDataToEndOfFile()
guard input.count <= 12 * 1024 * 1024,
      let request = try? JSONSerialization.jsonObject(with: input) as? [String: String],
      let destination = request["destination"] else { fail("Invalid request") }
if destination == "copy-card" {
    guard let raw = request["png"], let data = Data(base64Encoded: raw),
          data.count <= 8 * 1024 * 1024, NSImage(data: data) != nil else { fail("Invalid PNG") }
    let item = NSPasteboardItem()
    guard item.setData(data, forType: .png) else { fail("Cannot prepare image") }
    let board = NSPasteboard.general
    board.clearContents()
    guard board.writeObjects([item]) else { fail("Cannot copy image") }
    exit(0)
}
if destination == "telegram-probe" {
    exit(NSWorkspace.shared.urlForApplication(toOpen: URL(string: "tg://msg_url")!) == nil ? 1 : 0)
}
// Read-only diagnostics: enumerate service eligibility without showing or performing UI.
if destination == "wechat-probe" {
    _ = NSApplication.shared
    let url = URL(string: "https://www.xiaoyuzhoufm.com/episode/6aeb10f797726cd4675cbe17")!
    var result: [String: Bool] = [:]
    for (label, items): (String, [Any]) in [("url", [url]), ("textAndURL", ["test", url]), ("combinedText", ["test\n" + url.absoluteString])] {
        _ = NSSharingService.sharingServices(forItems: items)
        let service = NSSharingService(named: NSSharingService.Name(rawValue: "com.tencent.xinWeChat.WeChatMacShare"))
        result[label] = service?.canPerform(withItems: items) ?? false
    }
    let data = try! JSONSerialization.data(withJSONObject: result, options: [.sortedKeys])
    FileHandle.standardOutput.write(data)
    exit(0)
}
guard destination == "wechat" || destination == "system" else { fail("Unsupported destination") }
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
var items: [Any] = []
if destination == "wechat" {
    guard let raw = request["url"], let url = URL(string: raw),
          url.scheme == "https", url.host == "www.xiaoyuzhoufm.com",
          url.path.hasPrefix("/episode/") else { fail("Invalid episode URL") }
    items = [url]
} else {
    guard let raw = request["file"], raw.hasPrefix("/"), raw.hasSuffix(".png"),
          let data = try? Data(contentsOf: URL(fileURLWithPath:raw)),
          data.count <= 8 * 1024 * 1024, NSImage(data:data) != nil else { fail("Invalid PNG") }
    items = [URL(fileURLWithPath:raw)]
}
final class ShareUI: NSObject, NSSharingServicePickerDelegate, NSSharingServiceDelegate, NSWindowDelegate {
    var picker: NSSharingServicePicker?
    let wechat: Bool
    var selected: NSSharingService?
    init(wechat: Bool) { self.wechat = wechat }
    func sharingServicePicker(_ picker: NSSharingServicePicker, sharingServicesForItems items: [Any], proposedSharingServices proposedServices: [NSSharingService]) -> [NSSharingService] {
        if !wechat { return proposedServices }
        guard let service = wechatService, service.canPerform(withItems: items) else { return [] }
        return [service]
    }
    func sharingServicePicker(_ picker: NSSharingServicePicker, didChoose service: NSSharingService?) {
        guard let service = service else { NSApp.terminate(nil); return }
        selected = service
        service.delegate = self
    }
    func sharingService(_ sharingService: NSSharingService, didShareItems items: [Any]) { NSApp.terminate(nil) }
    func sharingService(_ sharingService: NSSharingService, didFailToShareItems items: [Any], error: Error) { fail(error.localizedDescription) }
    @objc func showPicker(_ sender: NSButton) {
        picker?.show(relativeTo: sender.bounds, of: sender, preferredEdge: .minY)
    }
    func windowWillClose(_ notification: Notification) { NSApp.terminate(nil) }
}
var wechatService: NSSharingService?
if destination == "wechat" {
    _ = NSSharingService.sharingServices(forItems: items)
    if let service = NSSharingService(named: NSSharingService.Name(rawValue: "com.tencent.xinWeChat.WeChatMacShare")), service.canPerform(withItems: items) {
        wechatService = service
    }
    guard wechatService != nil else { fail("微信分享不可用，请安装或启用微信分享扩展") }
}
let delegate = ShareUI(wechat: destination == "wechat")
let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 300, height: 110), styleMask: [.titled, .closable], backing: .buffered, defer: false)
window.title = "分享 · 请选择分享服务"
window.delegate = delegate
window.center()
let picker = NSSharingServicePicker(items: items)
picker.delegate = delegate
delegate.picker = picker
let button = NSButton(title: "选择分享服务…", target: delegate, action: #selector(ShareUI.showPicker(_:)))
button.frame = NSRect(x: 75, y: 35, width: 150, height: 32)
window.contentView!.addSubview(button)
window.makeKeyAndOrderFront(nil)
app.activate(ignoringOtherApps: true)
DispatchQueue.main.async {
    delegate.showPicker(button)
    FileHandle.standardOutput.write(Data("READY\n".utf8))
}
// AppKit extension cancellation does not always deliver a delegate callback.
// Bound the helper lifetime even in that case; Go has an independent deadline.
Timer.scheduledTimer(withTimeInterval: 120, repeats: false) { _ in fail("分享已超时关闭") }
let originalParent = getppid()
Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { _ in
    if getppid() != originalParent { NSApp.terminate(nil) }
}
app.run()
