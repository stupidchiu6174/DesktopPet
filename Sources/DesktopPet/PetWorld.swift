import AppKit
import CoreGraphics

/// 桌寵能站的地方:地面(Dock 上緣)+ 一般視窗看得到的上緣。
/// 照 Unity 版 WindowLedgeFeederNative 的邏輯:CGWindowList 天生由前到後排序,
/// 每個視窗的上緣用「在它前面的視窗」裁切,只有看得見的區段才算。
/// 只讀視窗的位置大小,不讀標題 → 不需要螢幕錄製權限。
struct PetLedge: Equatable {
    /// 地面是 "ground";視窗是 "<視窗編號>:<區段>"
    let id: String
    /// 上緣高度(螢幕座標,原點在主螢幕左下)
    let y: CGFloat
    let xMin: CGFloat
    let xMax: CGFloat
    /// 視窗本身的左緣:視窗被拖動時,她跟著水平移動同樣的距離
    let anchorX: CGFloat

    var isGround: Bool { id == "ground" }
    func clamp(_ x: CGFloat) -> CGFloat { min(max(x, xMin), xMax) }
    func random() -> CGFloat { .random(in: xMin...max(xMin, xMax)) }
}

enum PetWorld {

    /// 不當落腳處的系統元件
    private static let ignoredOwners: Set<String> = [
        "Window Server", "Dock", "Spotlight", "Control Center", "Control Centre",
        "Notification Center", "Notification Centre", "SystemUIServer", "WindowManager",
        "Screenshot", "TextInputMenuAgent", "Wallpaper",
    ]
    private static let minWidth: CGFloat = 200, minHeight: CGFloat = 100
    private static let maxSources = 6
    private static let minSegment: CGFloat = 150
    /// 太高的上緣不去(她的頭會撞到選單列)
    private static let headroom: CGFloat = 190

    /// 地面:這個螢幕可用區域的底(Dock 上面),兩側留一點邊
    static func ground(on screen: NSScreen) -> PetLedge {
        let f = screen.visibleFrame
        return PetLedge(id: "ground", y: f.minY, xMin: f.minX + 60, xMax: f.maxX - 60, anchorX: f.minX)
    }

    /// 這個螢幕上所有能站的地方(第 0 個永遠是地面)
    static func ledges(on screen: NSScreen) -> [PetLedge] {
        var result = [ground(on: screen)]
        guard let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements],
                                                    kCGNullWindowID) as? [[String: Any]] else { return result }
        // CG 座標原點在主螢幕左上,換成 AppKit 的左下原點
        let primaryHeight = NSScreen.screens.first?.frame.height ?? screen.frame.height
        let visible = screen.visibleFrame

        struct Win { let number: Int; let rect: CGRect; let source: Bool }
        var wins: [Win] = []
        var sources = 0
        for info in list where wins.count < 24 {
            guard (info[kCGWindowLayer as String] as? Int) == 0,
                  let owner = info[kCGWindowOwnerName as String] as? String, !ignoredOwners.contains(owner),
                  (info[kCGWindowAlpha as String] as? Double ?? 1) >= 0.5,
                  let boundsDict = info[kCGWindowBounds as String] as? NSDictionary,
                  let rect = CGRect(dictionaryRepresentation: boundsDict),
                  let number = info[kCGWindowNumber as String] as? Int else { continue }
            let big = rect.width >= minWidth && rect.height >= minHeight
            let source = big && sources < maxSources
            if source { sources += 1 }
            wins.append(Win(number: number, rect: rect, source: source))
        }

        for (i, win) in wins.enumerated() where win.source {
            let top = primaryHeight - win.rect.minY
            // 要在這個螢幕上、離地面有點高度、頭上還有空間
            guard top > visible.minY + 40, top + headroom < visible.maxY,
                  win.rect.maxX > visible.minX, win.rect.minX < visible.maxX else { continue }
            var segments = [(win.rect.minX, win.rect.maxX)]
            for front in wins[..<i] where win.rect.minY > front.rect.minY - 1 && win.rect.minY < front.rect.maxY - 1 {
                segments = subtract(segments, front.rect.minX, front.rect.maxX)
            }
            for (s, seg) in segments.enumerated() {
                let lo = max(seg.0, visible.minX) + 30, hi = min(seg.1, visible.maxX) - 30
                guard hi - lo >= minSegment - 60 else { continue }
                result.append(PetLedge(id: "\(win.number):\(s)", y: top, xMin: lo, xMax: hi, anchorX: win.rect.minX))
            }
        }
        return result
    }

    /// 一維線段減法:從 segments 挖掉 [a, b]
    private static func subtract(_ segments: [(CGFloat, CGFloat)], _ a: CGFloat, _ b: CGFloat) -> [(CGFloat, CGFloat)] {
        segments.flatMap { s -> [(CGFloat, CGFloat)] in
            guard b > s.0, a < s.1 else { return [s] }
            var out: [(CGFloat, CGFloat)] = []
            if a > s.0 { out.append((s.0, a)) }
            if b < s.1 { out.append((b, s.1)) }
            return out
        }
    }
}

/// 她的注意力(照 Unity 版 AttentionState):0…1 的內在值。
/// 大多時候很低——她過自己的生活。滑鼠在她面前、而且有在動,才慢慢累積;
/// 超過門檻才「轉過來看你」幾秒,之後有很長的冷卻,所以這件事很稀有。
struct PetAttention {
    enum Mode: Equatable { case selfDirected, spike(until: TimeInterval) }

    private(set) var value: Double = 0
    private(set) var mode: Mode = .selfDirected
    private var cooldownUntil: TimeInterval = 0
    private var lastMouse = CGPoint.zero
    private var mouseStillSince: TimeInterval = 0

    static let range: CGFloat = 520          // 「看得到」的距離(點)
    static let coneHalfAngle: Double = 40     // 側身時視線錐的半角(度)
    static let gain = 0.06, decay = 0.10, idleDecay = 0.05
    static let idleAfter: TimeInterval = 8
    static let threshold = 0.85
    static let spikeSeconds: TimeInterval = 3.5
    static let cooldown: TimeInterval = 90
    static let neglected = 0.15

    var isSpiking: Bool { if case .spike = mode { return true }; return false }

    /// 每格更新;回傳這格是否剛好結束一次 spike(給生活層「假裝沒事」換位置用)
    mutating func update(t: TimeInterval, dt: Double, mouse: CGPoint, head: CGPoint, facing: PetFacing) -> Bool {
        if (mouse.x - lastMouse.x) * (mouse.x - lastMouse.x) + (mouse.y - lastMouse.y) * (mouse.y - lastMouse.y) > 4 {
            mouseStillSince = t
        }
        lastMouse = mouse

        if case .spike(let until) = mode {
            if t < until { return false }
            mode = .selfDirected
            value = 0.2
            cooldownUntil = t + Self.cooldown
            return true
        }
        let idle = t - mouseStillSince > Self.idleAfter
        if inCone(mouse: mouse, head: head, facing: facing), !idle {
            value += Self.gain * dt
        } else {
            value -= Self.decay * dt
        }
        if idle { value -= Self.idleDecay * dt }
        value = min(max(value, 0), 1)
        if value >= Self.threshold, t >= cooldownUntil {
            mode = .spike(until: t + Self.spikeSeconds)
        }
        return false
    }

    /// 被直接互動(點她、抓她):無條件看你 seconds 秒——稀缺只管「沒被召喚」的注意力
    mutating func hold(_ seconds: TimeInterval, now t: TimeInterval) {
        if case .spike(let until) = mode {
            mode = .spike(until: max(until, t + seconds))
        } else {
            mode = .spike(until: t + seconds)
        }
    }

    private func inCone(mouse: CGPoint, head: CGPoint, facing: PetFacing) -> Bool {
        let dx = mouse.x - head.x, dy = mouse.y - head.y
        let distance = hypot(dx, dy)
        guard distance < Self.range, distance > 1 else { return false }
        switch facing {
        case .front:
            return dy > -160      // 正面:面前與上方都算,腳底下不算
        case .left, .right:
            let forward: CGFloat = facing == .left ? -1 : 1
            let angle = acos(Double(dx * forward) / Double(distance)) * 180 / .pi
            return angle <= Self.coneHalfAngle
        }
    }
}

enum PetFacing { case front, left, right }

/// 眨眼節奏(照 Unity 版 BlinkController):三角分布抽間隔——大多 3.5 秒左右、偶爾連眨、偶爾放空,
/// 絕不等間隔(等間隔比不眨還詭異)。閉眼圖顯示約 0.15 秒。
struct PetBlink {
    private var next: TimeInterval = 0

    mutating func isClosed(at t: TimeInterval) -> Bool {
        if next == 0 { next = t + Self.sample() }
        if t < next { return false }
        if t < next + 0.15 { return true }
        next = t + Self.sample()
        return false
    }

    private static func sample(min: Double = 1.2, mode: Double = 3.5, max: Double = 8) -> Double {
        let u = Double.random(in: 0..<1), c = (mode - min) / (max - min)
        return u < c ? min + sqrt(u * (max - min) * (mode - min))
                     : max - sqrt((1 - u) * (max - min) * (max - mode))
    }
}
