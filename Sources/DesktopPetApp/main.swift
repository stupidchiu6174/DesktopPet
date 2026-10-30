import AppKit
import DesktopPet

// 單獨執行桌寵:沒有 Dock 圖示,右鍵桌寵有選單
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
MainActor.assumeIsolated {
    if !PetController.shared.isEnabled { PetController.shared.toggle() }
}
app.run()
