import AppKit
import SwiftUI

/// 桌寵的衣服(素材在 Sources/DesktopPet/Resources/Pet/<衣服>/<姿勢>.png;tools/pet/make_sprites.sh 去背、裁切後放進來)
enum PetOutfit: String, CaseIterable {
    case witch, qipao, kimono

    var title: String {
        switch self {
        case .witch: "魔女裝"
        case .qipao: "旗袍"
        case .kimono: "和服"
        }
    }

    var greeting: String {
        switch self {
        case .witch: "魔女裝最自在了～"
        case .qipao: "換上旗袍了！好看嗎？"
        case .kimono: "和服…走路要小步一點"
        }
    }
}

enum PetPose: String, CaseIterable {
    case stand, blink, walk, sit, sleep, happy, surprised, held, dance
}

/// 素材快取。缺的姿勢退回站姿;另外留一份像素資料給「點到的是不是角色本身」的判斷。
@MainActor
final class PetSprites {

    static let shared = PetSprites()

    /// 素材像素 → 畫面點數(素材以 3 倍解析度生成,站姿約 160pt 高)
    static let scale: CGFloat = 1.0 / 3.0

    private var images: [String: NSImage] = [:]
    private var bitmaps: [ObjectIdentifier: NSBitmapImageRep] = [:]

    /// 開發時可以把新素材放在 ~/Library/Application Support/LiriOS/Pet/,優先讀這裡
    private var roots: [URL] {
        let user = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent("Library/Application Support/LiriOS/Pet")
        return [user, Bundle.main.resourceURL?.appendingPathComponent("Pet"),
                Bundle.module.resourceURL?.appendingPathComponent("Pet")].compactMap { $0 }
    }

    func image(_ outfit: PetOutfit, _ pose: PetPose, frame: Int = 0) -> NSImage? {
        // 走路有循環格(walk1…walkN)就輪播,沒有就用單張 walk
        if pose == .walk, case let frames = walkFrames(outfit), frames > 0,
           let image = load(outfit, "walk\(frame % frames + 1)") {
            return image
        }
        return load(outfit, pose.rawValue) ?? load(outfit, PetPose.stand.rawValue)
            ?? load(.witch, pose.rawValue) ?? load(.witch, PetPose.stand.rawValue)
    }

    private var frameCounts: [PetOutfit: Int] = [:]

    /// 這套衣服有幾格走路循環
    func walkFrames(_ outfit: PetOutfit) -> Int {
        if let n = frameCounts[outfit] { return n }
        var n = 0
        while load(outfit, "walk\(n + 1)") != nil { n += 1 }
        frameCounts[outfit] = n
        return n
    }

    private func load(_ outfit: PetOutfit, _ name: String) -> NSImage? {
        let key = "\(outfit.rawValue)/\(name)"
        if let cached = images[key] { return cached }
        for root in roots {
            let url = root.appendingPathComponent("\(key).png")
            if let image = NSImage(contentsOf: url), let rep = image.representations.first {
                // 以像素大小 × scale 當點數大小(不管 PNG 標的 DPI)
                image.size = NSSize(width: CGFloat(rep.pixelsWide) * Self.scale,
                                    height: CGFloat(rep.pixelsHigh) * Self.scale)
                images[key] = image
                return image
            }
        }
        return nil
    }

    /// 圖上某一點(以圖的左下為原點、點數為單位)是不是不透明
    func isOpaque(_ image: NSImage, at point: CGPoint) -> Bool {
        let id = ObjectIdentifier(image)
        let rep: NSBitmapImageRep
        if let cached = bitmaps[id] {
            rep = cached
        } else {
            guard let tiff = image.tiffRepresentation, let made = NSBitmapImageRep(data: tiff) else { return true }
            bitmaps[id] = made
            rep = made
        }
        let x = Int(point.x / image.size.width * CGFloat(rep.pixelsWide))
        let y = Int((1 - point.y / image.size.height) * CGFloat(rep.pixelsHigh))
        guard x >= 0, y >= 0, x < rep.pixelsWide, y < rep.pixelsHigh else { return false }
        return (rep.colorAt(x: x, y: y)?.alphaComponent ?? 0) > 0.15
    }
}

/// 畫面要顯示的狀態(PetController 每格更新)
@MainActor
@Observable
final class PetModel {
    var outfit: PetOutfit = .witch
    var pose: PetPose = .stand
    var facingLeft = false
    /// 往上跳的高度(點)
    var bob: CGFloat = 0
    /// 左右搖擺角度(度)
    var tilt: Double = 0
    /// 上下壓扁(1 = 原狀;落地時 < 1)
    var squash: CGFloat = 1
    var bubble: String?
    /// 換衣服時的小特效
    var poof = false

    /// 走路循環現在是第幾格
    var walkFrame = 0

    var image: NSImage? { PetSprites.shared.image(outfit, pose, frame: walkFrame) }
}

/// 桌寵的樣子:角色站在視窗底部中間,對話泡泡在頭上
struct PetView: View {
    let model: PetModel

    var body: some View {
        ZStack(alignment: .bottom) {
            if let image = model.image {
                Image(nsImage: image)
                    .interpolation(.high)
                    .scaleEffect(x: model.facingLeft ? -1 : 1, y: 1)
                    .scaleEffect(x: 2 - model.squash, y: model.squash, anchor: .bottom)
                    .rotationEffect(.degrees(model.tilt), anchor: .bottom)
                    .offset(y: -model.bob)
                    .scaleEffect(model.poof ? 0.6 : 1, anchor: .bottom)
                    .opacity(model.poof ? 0.3 : 1)
                    .animation(.spring(duration: 0.35, bounce: 0.4), value: model.poof)
                    .shadow(color: .black.opacity(0.18), radius: 3, y: 2)
                    .overlay(alignment: .top) {
                        if let text = model.bubble {
                            SpeechBubble(text: text)
                                .fixedSize()
                                .offset(y: -44 - model.bob)
                                .transition(.scale(scale: 0.6, anchor: .bottom).combined(with: .opacity))
                        }
                    }
                    .animation(.spring(duration: 0.3, bounce: 0.3), value: model.bubble)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
    }
}

private struct SpeechBubble: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.system(size: 13, weight: .medium, design: .rounded))
            .foregroundStyle(.black.opacity(0.85))
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .fill(.white)
                    .shadow(color: .black.opacity(0.2), radius: 3, y: 1)
            )
            .overlay(alignment: .bottom) {
                Triangle().fill(.white).frame(width: 12, height: 7).offset(y: 6)
            }
    }
}

private struct Triangle: Shape {
    func path(in rect: CGRect) -> Path {
        Path { p in
            p.move(to: CGPoint(x: rect.minX, y: rect.minY))
            p.addLine(to: CGPoint(x: rect.maxX, y: rect.minY))
            p.addLine(to: CGPoint(x: rect.midX, y: rect.maxY))
            p.closeSubpath()
        }
    }
}
