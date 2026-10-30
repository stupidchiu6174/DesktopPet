import AppKit
import SwiftUI

/// 桌寵:照 Unity 版「莉莉」(Descktop 專案)的生活邏輯,不含 LLM。
///
/// 她的世界 = 地面(Dock 上緣)+ 一般視窗看得到的上緣(PetWorld)。
/// 生活循環(DesktopLifeBrain):站著發呆 20–60 秒 → 七成走去某處坐 30–120 秒、三成散步
/// (散步時 35% 跳到別的視窗上)。腳下的視窗被拖動她會跟著移動,被拖走或關掉她就掉下去,
/// 途中可以被別的視窗接住。
/// 注意力(PetAttention):大多時候不理你;滑鼠在她面前晃久了才轉過來看你幾秒,之後冷卻很久;
/// 看完常常「假裝沒事」換個地方。被冷落時她換地方的節奏會快一倍。
///
/// 互動:按住拖曳 = 拎起來(放開從那裡掉下去,可以放到視窗上);點一下 = 看你、開心一下;
/// 點兩下 = 換下一套衣服;右鍵 = 換衣服 / 坐下 / 睡覺 / 收起來。
/// LiriOS 的部分:電腦放著五分鐘她就睡著、你回來會醒;有音樂在播就跳舞。
///
/// 省電(DesktopPerformance):在動的時候每秒 30 格,發呆時每秒 12 格;位置沒變就不動視窗。
/// 角色以外的透明區域點擊會穿透到下面的視窗。
@MainActor
public final class PetController {

    public static let shared = PetController()
    /// 目前有沒有在放音樂(有的話桌寵會跳舞);由主程式提供
    public var isMusicPlaying: () -> Bool = { false }

    private enum Life: Equatable {
        case standing(until: TimeInterval)
        case moving(to: CGFloat, sitAfter: Bool)
        case jumping(from: CGPoint, to: CGPoint, start: TimeInterval, duration: TimeInterval, ledge: String, sitAfter: Bool)
        case sitting(until: TimeInterval)
        case falling(velocity: CGFloat)
        case held
        case sleeping
        case dancing
        case reacting(PetPose, until: TimeInterval)
    }

    private static let keyEnabled = "pet.enabled"
    private static let keyOutfit = "pet.outfit"
    private static let windowSize = CGSize(width: 260, height: 260)

    public private(set) var isEnabled = false
    let model = PetModel()
    private var window: PetWindow?
    private var timer: Timer?
    private var rate: Double = 0
    private var life: Life = .standing(until: 0)
    private var attention = PetAttention()
    private var blink = PetBlink()
    /// 腳底中央的位置(螢幕座標)
    private var feet = CGPoint.zero
    private var ledges: [PetLedge] = []
    private var ledgeID = "ground"
    private var lastLedgePoll: TimeInterval = 0
    private var wakeSettleUntil: TimeInterval = 0
    private var last = CACurrentMediaTime()
    private var bubbleUntil: TimeInterval = 0
    private var lastOrigin = CGPoint(x: CGFloat.nan, y: CGFloat.nan)
    private var wakeObserver: NSObjectProtocol?

    // MARK: - 開關

    public func restore() {
        if let raw = UserDefaults.standard.string(forKey: Self.keyOutfit), let outfit = PetOutfit(rawValue: raw) {
            model.outfit = outfit
        }
        // 預設開
        if UserDefaults.standard.object(forKey: Self.keyEnabled) == nil || UserDefaults.standard.bool(forKey: Self.keyEnabled) {
            show()
        }
    }

    public func toggle() { isEnabled ? hide() : show() }

    func show() {
        guard window == nil, let screen = NSScreen.main else { return }
        let w = PetWindow(size: Self.windowSize)
        let host = PetHostingView(rootView: PetView(model: model))
        host.sizingOptions = []
        host.controller = self
        w.contentView = host
        window = w
        ledges = [PetWorld.ground(on: screen)]
        ledgeID = "ground"
        feet = CGPoint(x: screen.visibleFrame.maxX - 220, y: ledges[0].y)
        place()
        w.orderFrontRegardless()
        isEnabled = true
        UserDefaults.standard.set(true, forKey: Self.keyEnabled)
        enterStanding(now)
        last = now
        // 闔蓋睡醒的那一刻 window server 狀態還不穩:靜置兩秒再抓視窗(Unity 版在這裡當過)
        wakeObserver = NSWorkspace.shared.notificationCenter.addObserver(
            forName: NSWorkspace.didWakeNotification, object: nil, queue: .main
        ) { _ in
            MainActor.assumeIsolated {
                let c = PetController.shared
                c.wakeSettleUntil = c.now + 2
            }
        }
        // 量效能用:`notifyutil` 送不了分散式通知,用 swift 送 com.quewuyin.lirios.pet.drop 讓她從高處掉下來
        dropObserver = DistributedNotificationCenter.default().addObserver(
            forName: Notification.Name("com.quewuyin.lirios.pet.drop"), object: nil, queue: .main
        ) { _ in
            MainActor.assumeIsolated { PetController.shared.dropFromTop() }
        }
        setRate(30)
    }

    private var dropObserver: NSObjectProtocol?

    private func dropFromTop() {
        guard window != nil else { return }
        feet.y = screen.visibleFrame.maxY - 200
        ledges = PetWorld.ledges(on: screen)
        lastLedgePoll = now
        startFalling()
    }

    func hide() {
        timer?.invalidate()
        timer = nil
        rate = 0
        window?.orderOut(nil)
        window = nil
        if let wakeObserver { NSWorkspace.shared.notificationCenter.removeObserver(wakeObserver) }
        wakeObserver = nil
        if let dropObserver { DistributedNotificationCenter.default().removeObserver(dropObserver) }
        dropObserver = nil
        isEnabled = false
        UserDefaults.standard.set(false, forKey: Self.keyEnabled)
    }

    func setOutfit(_ outfit: PetOutfit) {
        guard outfit != model.outfit else { return }
        model.poof = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.18) {
            MainActor.assumeIsolated {
                let c = PetController.shared
                c.model.outfit = outfit
                c.model.poof = false
                c.say(outfit.greeting)
            }
        }
        UserDefaults.standard.set(outfit.rawValue, forKey: Self.keyOutfit)
    }

    /// 動的時候 30fps、發呆 12fps(呼吸和眨眼在 12fps 仍然自然)
    private func setRate(_ hz: Double) {
        guard hz != rate else { return }
        rate = hz
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 1 / hz, repeats: true) { _ in
            MainActor.assumeIsolated { PetController.shared.tick() }
        }
    }

    // MARK: - 世界

    private var now: TimeInterval { CACurrentMediaTime() }

    private var screen: NSScreen {
        NSScreen.screens.first { $0.frame.contains(CGPoint(x: feet.x, y: feet.y + 1)) } ?? NSScreen.main ?? NSScreen.screens[0]
    }

    private var currentLedge: PetLedge? { ledges.first { $0.id == ledgeID } }

    /// 重抓視窗邊沿。站在視窗上時每秒 10 次(要跟得上被拖的視窗),在地上每秒 1 次就夠
    private func pollLedges(_ t: TimeInterval) {
        guard t >= wakeSettleUntil else { return }
        let onWindow = ledgeID != "ground"
        let interval: TimeInterval
        switch life {
        case .falling, .held, .jumping: interval = 0.1
        default: interval = onWindow ? 0.1 : 1.0
        }
        guard t - lastLedgePoll >= interval else { return }
        lastLedgePoll = t
        let old = currentLedge
        ledges = PetWorld.ledges(on: screen)

        switch life {
        case .jumping, .falling, .held: return
        default: break
        }
        guard let ledge = currentLedge else {
            // 腳下的視窗被關掉 / 移走了:掉
            if let old, !old.isGround { startFalling() }
            ledgeID = "ground"
            return
        }
        // 視窗被拖著走:她騎在上面,水平跟著移動
        if let old, old.id == ledge.id { feet.x += ledge.anchorX - old.anchorX }
        // 支撐判定:腳下已經沒有這段邊沿 → 掉,不是滑到邊緣
        if feet.x < ledge.xMin - 20 || feet.x > ledge.xMax + 20 {
            startFalling()
            return
        }
        feet.y = ledge.y
    }

    // MARK: - 每一格

    private func tick() {
        let t = now
        let dt = min(t - last, 0.15)
        last = t
        if model.bubble != nil, t > bubbleUntil { model.bubble = nil }

        pollLedges(t)
        lifeEvents(t)

        // 注意力:頭大約在腳上方 130pt
        let facing: PetFacing = model.pose == .walk ? (model.facingLeft ? .left : .right) : .front
        let spikeEnded = attention.update(t: t, dt: dt, mouse: NSEvent.mouseLocation,
                                          head: CGPoint(x: feet.x, y: feet.y + 130), facing: facing)
        if spikeEnded { afterSpike(t) }

        step(t, CGFloat(dt))

        // 站著時眨眼
        if model.pose == .stand, blink.isClosed(at: t) { model.pose = .blink }

        // 呼吸 / 落地壓扁慢慢回彈
        let breathe: CGFloat = life == .sleeping ? 0.02 : 0.008
        let target = 1 + breathe * CGFloat(sin(t * 2.2))
        model.squash += (target - model.squash) * min(1, 10 * CGFloat(dt))

        place()
        updateClickThrough()
        setRate(isActive ? 30 : 12)
    }

    private var isActive: Bool {
        if attention.isSpiking { return true }
        switch life {
        case .moving, .jumping, .falling, .held, .dancing, .reacting: return true
        default: return false
        }
    }

    /// 生活以外的事:你離開太久就睡、回來就醒;音樂在播就跳舞
    private func lifeEvents(_ t: TimeInterval) {
        switch life {
        case .held, .falling, .jumping: return
        default: break
        }
        let idleSeconds = CGEventSource.secondsSinceLastEventType(.combinedSessionState, eventType: CGEventType(rawValue: ~0)!)
        if life == .sleeping {
            if idleSeconds < 1.5 {
                life = .reacting(.surprised, until: t + 1.2)
                say("…啊,你回來了")
            }
            return
        }
        if idleSeconds > 300 {
            life = .sleeping
            return
        }
        let playing = isMusicPlaying()
        if playing, life != .dancing, case .standing = life {
            life = .dancing
        } else if !playing, life == .dancing {
            enterStanding(t)
        }
    }

    private func step(_ t: TimeInterval, _ dt: CGFloat) {
        let spiking = attention.isSpiking
        model.walkFrame = 0

        switch life {
        case .standing(let until):
            model.pose = .stand
            model.bob = 0; model.tilt = 0
            if spiking { faceMouse(); return }
            // 被冷落時,發呆的時間過得比較快(每格多扣一份 = 兩倍速)
            var deadline = until
            if attention.value < PetAttention.neglected {
                deadline -= Double(dt)
                life = .standing(until: deadline)
            }
            // 七成找地方坐(不換邊),三成純散步(可能跳到別的視窗)
            if t >= deadline { decideNewSpot(t, sitAfter: Double.random(in: 0..<1) < 0.7) }

        case .moving(let target, let sitAfter):
            if spiking {
                // 轉過來看你的時候停下腳步,不原地踏步
                model.pose = .stand; model.bob = 0; model.tilt = 0
                faceMouse()
                return
            }
            let dx = target - feet.x
            let speed: CGFloat = model.outfit == .kimono ? 32 : 45
            if abs(dx) < 2 {
                if sitAfter { enterSitting(t) } else { enterStanding(t) }
                return
            }
            model.facingLeft = dx < 0
            feet.x += max(-speed * dt, min(speed * dt, dx))
            if let ledge = currentLedge { feet.y = ledge.y }
            model.pose = .walk
            if PetSprites.shared.walkFrames(model.outfit) > 0 {
                model.walkFrame = Int(t * 8 * Double(speed) / 45)
                model.bob = abs(sin(t * .pi * 4)) * 1.5
                model.tilt = 0
            } else {
                model.bob = abs(sin(t * .pi * 4)) * 4
                model.tilt = sin(t * .pi * 4) * 2
            }

        case .jumping(let from, let to, let start, let duration, let target, let sitAfter):
            let p = min(1, (t - start) / duration)
            let arc = max(70, abs(to.y - from.y) * 0.35)
            feet = CGPoint(x: from.x + (to.x - from.x) * p,
                           y: from.y + (to.y - from.y) * p + arc * 4 * p * (1 - p))
            model.pose = .happy
            model.bob = 0; model.tilt = 0
            if p >= 1 {
                feet = to
                ledgeID = target
                model.squash = 0.9
                if sitAfter { enterSitting(t) } else { enterStanding(t) }
            }

        case .sitting(let until):
            model.pose = .sit
            model.bob = 0; model.tilt = 0
            if spiking { faceMouse(); return }
            if t > until { enterStanding(t) }

        case .falling(let velocity):
            let v = velocity - 2200 * dt
            let previous = feet.y
            feet.y += v * dt
            model.pose = .surprised
            model.bob = 0; model.tilt = 0
            // 這一格的掉落路徑上、接得住她的最高邊沿(地面永遠接得住)
            let catcher = ledges.filter { l in
                (l.isGround || (feet.x >= l.xMin && feet.x <= l.xMax)) && l.y <= previous + 0.5 && l.y >= feet.y
            }.max { $0.y < $1.y }
            if let catcher {
                feet.y = catcher.y
                if catcher.isGround { feet.x = catcher.clamp(feet.x) }
                ledgeID = catcher.id
                let impact = -v
                model.squash = impact > 600 ? 0.8 : 0.92
                if impact > 900 {
                    life = .reacting(.surprised, until: t + 1.0)
                    say(["哎唷!", "好痛…"].randomElement()!)
                } else {
                    enterStanding(t)
                }
            } else {
                life = .falling(velocity: v)
            }

        case .held:
            model.pose = .held
            model.tilt = sin(t * 3) * 6
            model.bob = 0

        case .sleeping:
            model.pose = .sleep
            model.bob = 0; model.tilt = 0
            if model.bubble == nil, Int(t) % 7 == 0 { say("Zz…", for: 2) }

        case .dancing:
            model.pose = .dance
            model.bob = abs(sin(t * .pi * 4)) * 8
            model.tilt = sin(t * .pi * 2) * 6

        case .reacting(let pose, let until):
            model.pose = pose
            model.bob = pose == .happy ? abs(sin(t * .pi * 3)) * 12 : 0
            model.tilt = 0
            if t > until { enterStanding(t) }
        }
    }

    // MARK: - 生活決策(DesktopLifeBrain)

    private func enterStanding(_ t: TimeInterval) {
        life = .standing(until: t + .random(in: 20...60))
    }

    private func enterSitting(_ t: TimeInterval) {
        life = .sitting(until: t + .random(in: 30...120))
    }

    /// 換個地方:要坐就在住的這條邊上找位子;純散步才可能跳到別的視窗
    private func decideNewSpot(_ t: TimeInterval, sitAfter: Bool) {
        guard let ledge = currentLedge ?? ledges.first else { return }
        let others = ledges.filter { $0.id != ledge.id }
        if !sitAfter, !others.isEmpty, Double.random(in: 0..<1) < 0.35, let target = others.randomElement() {
            jump(to: target, sitAfter: Double.random(in: 0..<1) < 0.4, t)
            return
        }
        let span: CGFloat = 420
        let x = ledge.clamp(feet.x + .random(in: -span...span))
        life = .moving(to: x, sitAfter: sitAfter)
    }

    private func jump(to target: PetLedge, sitAfter: Bool, _ t: TimeInterval) {
        let to = CGPoint(x: target.clamp(feet.x + .random(in: -80...80)), y: target.y)
        let distance = hypot(to.x - feet.x, to.y - feet.y)
        model.facingLeft = to.x < feet.x
        life = .jumping(from: feet, to: to, start: t, duration: min(1.3, max(0.6, Double(distance) / 700)),
                        ledge: target.id, sitAfter: sitAfter)
    }

    /// spike 結束:常常「假裝沒事」換個位置
    private func afterSpike(_ t: TimeInterval) {
        switch life {
        case .standing, .sitting:
            if Double.random(in: 0..<1) < 0.6 { decideNewSpot(t, sitAfter: Double.random(in: 0..<1) < 0.5) }
        default: break
        }
    }

    private func faceMouse() {
        model.facingLeft = NSEvent.mouseLocation.x < feet.x
    }

    private func startFalling() {
        life = .falling(velocity: 0)
    }

    private func say(_ text: String, for seconds: TimeInterval = 2.5) {
        model.bubble = text
        bubbleUntil = now + seconds
    }

    // MARK: - 視窗

    private func place() {
        guard let window else { return }
        let size = Self.windowSize
        // 坐在視窗上緣時往下沉一點,像坐在牆頭上
        let sink: CGFloat = (model.pose == .sit && ledgeID != "ground") ? 6 : 0
        let origin = CGPoint(x: (feet.x - size.width / 2).rounded(), y: (feet.y - sink).rounded())
        guard origin != lastOrigin else { return }   // 沒動就不搬視窗(省 WindowServer 的合成)
        lastOrigin = origin
        window.setFrameOrigin(origin)
    }

    /// 滑鼠在角色身上才接收點擊,其他地方穿透到下面的視窗
    private func updateClickThrough() {
        guard let window else { return }
        if life == .held { window.ignoresMouseEvents = false; return }
        let ignore = !hitsCharacter(NSEvent.mouseLocation)
        if window.ignoresMouseEvents != ignore { window.ignoresMouseEvents = ignore }
    }

    private func hitsCharacter(_ screenPoint: CGPoint) -> Bool {
        guard let window, let image = model.image else { return false }
        let local = CGPoint(x: screenPoint.x - window.frame.minX, y: screenPoint.y - window.frame.minY - model.bob)
        let size = image.size
        let originX = (Self.windowSize.width - size.width) / 2
        var p = CGPoint(x: local.x - originX, y: local.y)
        guard p.x >= 0, p.y >= 0, p.x < size.width, p.y < size.height else { return false }
        if model.facingLeft { p.x = size.width - p.x }
        return PetSprites.shared.isOpaque(image, at: p)
    }

    // MARK: - 滑鼠(PetHostingView 轉過來)

    private var pressedAt = CGPoint.zero
    private var dragged = false
    private var grabOffset = CGPoint.zero

    func mouseDown(_ event: NSEvent) {
        pressedAt = NSEvent.mouseLocation
        dragged = false
    }

    func mouseDragged(_ event: NSEvent) {
        let p = NSEvent.mouseLocation
        if !dragged, hypot(p.x - pressedAt.x, p.y - pressedAt.y) < 4 { return }
        if !dragged {
            dragged = true
            life = .held
            attention.hold(2, now: now)
            // 拎著頭:腳底跟在滑鼠下方
            grabOffset = CGPoint(x: 0, y: (model.image?.size.height ?? 150) * 0.85)
            setRate(30)
        }
        attention.hold(1, now: now)
        feet = CGPoint(x: p.x - grabOffset.x, y: p.y - grabOffset.y)
        place()
    }

    func mouseUp(_ event: NSEvent) {
        if dragged {
            // 放開:從這個高度掉下去,被視窗或地面接住
            ledges = PetWorld.ledges(on: screen)
            lastLedgePoll = now
            startFalling()
            return
        }
        if event.clickCount >= 2 {
            let all = PetOutfit.allCases
            setOutfit(all[(all.firstIndex(of: model.outfit)! + 1) % all.count])
            return
        }
        if life == .sleeping {
            life = .reacting(.surprised, until: now + 1.2)
            say("嗯…?我沒有在睡喔")
            return
        }
        // 被你點了:看你一下,開心一下
        attention.hold(3, now: now)
        switch life {
        case .standing, .moving, .dancing, .reacting:
            life = .reacting(.happy, until: now + 1.2)
        default: break
        }
    }

    func menu() -> NSMenu {
        let menu = NSMenu()
        let outfits = NSMenuItem(title: "換衣服", action: nil, keyEquivalent: "")
        let sub = NSMenu()
        for outfit in PetOutfit.allCases {
            let item = PetMenuItem(title: outfit.title) { PetController.shared.setOutfit(outfit) }
            item.state = outfit == model.outfit ? .on : .off
            sub.addItem(item)
        }
        outfits.submenu = sub
        menu.addItem(outfits)
        menu.addItem(PetMenuItem(title: "坐下休息") {
            let c = PetController.shared
            c.enterSitting(c.now)
        })
        menu.addItem(PetMenuItem(title: "去睡覺") {
            let c = PetController.shared
            c.life = .sleeping
            c.say("晚安～", for: 2)
        })
        menu.addItem(.separator())
        menu.addItem(PetMenuItem(title: "收起桌寵") { PetController.shared.hide() })
        return menu
    }
}

/// 桌寵視窗:透明、不搶焦點、所有桌面都在、浮在一般視窗上面
final class PetWindow: NSPanel {
    init(size: CGSize) {
        super.init(contentRect: NSRect(origin: .zero, size: size), styleMask: [.borderless, .nonactivatingPanel],
                   backing: .buffered, defer: false)
        level = .floating
        collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle]
        isOpaque = false
        backgroundColor = .clear
        hasShadow = false
        hidesOnDeactivate = false
        isReleasedWhenClosed = false
        animationBehavior = .none
        becomesKeyOnlyIfNeeded = true
    }
    override var canBecomeKey: Bool { false }
    override func constrainFrameRect(_ frameRect: NSRect, to screen: NSScreen?) -> NSRect { frameRect }
}

final class PetHostingView: NSHostingView<PetView> {
    weak var controller: PetController?

    override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }
    override func mouseDown(with event: NSEvent) { controller?.mouseDown(event) }
    override func mouseDragged(with event: NSEvent) { controller?.mouseDragged(event) }
    override func mouseUp(with event: NSEvent) { controller?.mouseUp(event) }
    override func rightMouseDown(with event: NSEvent) {
        guard let controller else { return }
        NSMenu.popUpContextMenu(controller.menu(), with: event, for: self)
    }
}

/// 帶閉包的選單項目
private final class PetMenuItem: NSMenuItem {
    private let handler: @MainActor () -> Void

    init(title: String, handler: @escaping @MainActor () -> Void) {
        self.handler = handler
        super.init(title: title, action: #selector(run), keyEquivalent: "")
        target = self
    }

    required init(coder: NSCoder) { fatalError() }

    @objc private func run() { MainActor.assumeIsolated { handler() } }
}
