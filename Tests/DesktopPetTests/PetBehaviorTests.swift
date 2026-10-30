import AppKit
import XCTest
@testable import DesktopPet

final class PetBehaviorTests: XCTestCase {

    private let head = CGPoint(x: 500, y: 300)

    /// 滑鼠一直在她面前晃:大約 14 秒才轉過來,看 3.5 秒,之後冷卻 90 秒,冷卻完才會再轉
    func testAttentionSpikeIsRareAndCoolsDown() {
        var a = PetAttention()
        var t: TimeInterval = 0
        var spikeStarts: [TimeInterval] = []
        var spikeEnded: TimeInterval?
        var wasSpiking = false
        while t < 150 {
            t += 0.1
            // 在她面前 200pt 處小幅晃動(有在動,不算閒置)
            let mouse = CGPoint(x: head.x + 200 + CGFloat(sin(t * 3)) * 10, y: head.y + 20)
            if a.update(t: t, dt: 0.1, mouse: mouse, head: head, facing: .front) { spikeEnded = spikeEnded ?? t }
            if a.isSpiking, !wasSpiking { spikeStarts.append(t) }
            wasSpiking = a.isSpiking
        }
        XCTAssertEqual(spikeStarts.count, 2)
        XCTAssertEqual(spikeStarts[0], 0.85 / 0.06, accuracy: 0.5)
        XCTAssertEqual(spikeEnded! - spikeStarts[0], 3.5, accuracy: 0.2)
        XCTAssertEqual(spikeStarts[1] - spikeEnded!, 90, accuracy: 0.3, "冷卻 90 秒內不能再轉過來")
    }

    func testMouseBehindHerOrIdleDoesNotCount() {
        var a = PetAttention()
        var t: TimeInterval = 0
        // 側身朝右,滑鼠在她左後方
        for _ in 0..<300 {
            t += 0.1
            _ = a.update(t: t, dt: 0.1, mouse: CGPoint(x: head.x - 200 + CGFloat(sin(t * 3)) * 10, y: head.y),
                         head: head, facing: .right)
        }
        XCTAssertEqual(a.value, 0, accuracy: 0.001)
        // 正面、滑鼠不動:8 秒後算閒置,不再累積
        var b = PetAttention()
        t = 0
        for _ in 0..<300 {
            t += 0.1
            _ = b.update(t: t, dt: 0.1, mouse: CGPoint(x: head.x + 100, y: head.y), head: head, facing: .front)
        }
        XCTAssertFalse(b.isSpiking)
        XCTAssertLessThan(b.value, 0.2)
    }

    func testHoldGazeAlwaysWorks() {
        var a = PetAttention()
        a.hold(3, now: 10)
        XCTAssertTrue(a.isSpiking)
        XCTAssertFalse(a.update(t: 12, dt: 0.1, mouse: .zero, head: head, facing: .front))
        XCTAssertTrue(a.update(t: 13.1, dt: 0.1, mouse: .zero, head: head, facing: .front))
    }

    /// 眨眼間隔不等距,而且落在 1.2–8 秒之間
    func testBlinkIntervalsAreIrregular() {
        var blink = PetBlink()
        var t: TimeInterval = 0
        var starts: [TimeInterval] = []
        var closed = false
        while t < 600 {
            t += 0.02
            let now = blink.isClosed(at: t)
            if now, !closed { starts.append(t) }
            closed = now
        }
        let gaps = zip(starts.dropFirst(), starts).map { $0 - $1 }
        XCTAssertGreaterThan(gaps.count, 60)
        XCTAssertGreaterThanOrEqual(gaps.min()!, 1.2 + 0.15 - 0.05)
        XCTAssertLessThanOrEqual(gaps.max()!, 8 + 0.15 + 0.05)
        let mean = gaps.reduce(0, +) / Double(gaps.count)
        XCTAssertEqual(mean, (1.2 + 3.5 + 8) / 3 + 0.15, accuracy: 0.6)
    }

    /// 實際螢幕:地面一定在,視窗邊沿都在螢幕內、寬度夠
    @MainActor
    func testLedgesOnThisScreen() throws {
        let screen = try XCTUnwrap(NSScreen.main)
        let ledges = PetWorld.ledges(on: screen)
        XCTAssertEqual(ledges.first?.id, "ground")
        for l in ledges {
            XCTAssertLessThan(l.xMin, l.xMax)
            XCTAssertGreaterThanOrEqual(l.y, screen.visibleFrame.minY)
            XCTAssertLessThan(l.y, screen.visibleFrame.maxY)
            print("邊沿 \(l.id)  y=\(Int(l.y))  x=\(Int(l.xMin))…\(Int(l.xMax))")
        }
    }
}

final class PetWorldCostTests: XCTestCase {
    /// 抓一次視窗邊沿要多久(掉落、被拎著、站在視窗上時每秒 10 次)
    @MainActor
    func testLedgeScanCost() throws {
        let screen = try XCTUnwrap(NSScreen.main)
        _ = PetWorld.ledges(on: screen)
        let runs = 200
        let start = CFAbsoluteTimeGetCurrent()
        for _ in 0..<runs { _ = PetWorld.ledges(on: screen) }
        let ms = (CFAbsoluteTimeGetCurrent() - start) / Double(runs) * 1000
        print("抓一次視窗邊沿 \(String(format: "%.2f", ms)) ms")
    }
}
