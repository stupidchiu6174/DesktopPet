package main

import (
	"math"
	"math/rand"
)

// 座標一律是「點」(像素 ÷ DPI 縮放),y 朝上(螢幕 y 取負),跟 macOS 版的數字可以直接對照。

type point struct{ x, y float64 }

// 桌寵能站的地方:地面(工作列上緣)+ 一般視窗看得到的上緣。
type ledge struct {
	// 地面是 "ground";視窗是 "<視窗 handle>:<區段>"
	id         string
	y          float64
	xMin, xMax float64
	// 視窗本身的左緣:視窗被拖動時,她跟著水平移動同樣的距離
	anchorX float64
}

func (l ledge) isGround() bool { return l.id == "ground" }

func (l ledge) clamp(x float64) float64 { return math.Min(math.Max(x, l.xMin), l.xMax) }

type rect struct{ minX, minY, maxX, maxY float64 }

func groundLedge(work rect) ledge {
	return ledge{id: "ground", y: work.minY, xMin: work.minX + 60, xMax: work.maxX - 60, anchorX: work.minX}
}

// 一維線段減法:從 segments 挖掉 [a, b]
func subtract(segments [][2]float64, a, b float64) [][2]float64 {
	var out [][2]float64
	for _, s := range segments {
		if b <= s[0] || a >= s[1] {
			out = append(out, s)
			continue
		}
		if a > s[0] {
			out = append(out, [2]float64{s[0], a})
		}
		if b < s[1] {
			out = append(out, [2]float64{b, s[1]})
		}
	}
	return out
}

// 她的注意力:0…1 的內在值。大多時候很低——她過自己的生活。
// 滑鼠在她面前、而且有在動,才慢慢累積;超過門檻才「轉過來看你」幾秒,之後有很長的冷卻。
type attention struct {
	value          float64
	spiking        bool
	spikeUntil     float64
	cooldownUntil  float64
	lastMouse      point
	mouseStillFrom float64
}

const (
	attRange         = 520.0
	attConeHalfAngle = 40.0
	attGain          = 0.06
	attDecay         = 0.10
	attIdleDecay     = 0.05
	attIdleAfter     = 8.0
	attThreshold     = 0.85
	attSpikeSeconds  = 3.5
	attCooldown      = 90.0
	attNeglected     = 0.15
)

type facing int

const (
	faceFront facing = iota
	faceLeft
	faceRight
)

// 每格更新;回傳這格是否剛好結束一次 spike
func (a *attention) update(t, dt float64, mouse, head point, f facing) bool {
	if dx, dy := mouse.x-a.lastMouse.x, mouse.y-a.lastMouse.y; dx*dx+dy*dy > 4 {
		a.mouseStillFrom = t
	}
	a.lastMouse = mouse

	if a.spiking {
		if t < a.spikeUntil {
			return false
		}
		a.spiking = false
		a.value = 0.2
		a.cooldownUntil = t + attCooldown
		return true
	}
	idle := t-a.mouseStillFrom > attIdleAfter
	if inCone(mouse, head, f) && !idle {
		a.value += attGain * dt
	} else {
		a.value -= attDecay * dt
	}
	if idle {
		a.value -= attIdleDecay * dt
	}
	a.value = math.Min(math.Max(a.value, 0), 1)
	if a.value >= attThreshold && t >= a.cooldownUntil {
		a.spiking = true
		a.spikeUntil = t + attSpikeSeconds
	}
	return false
}

// 被直接互動(點她、抓她):無條件看你 seconds 秒
func (a *attention) hold(seconds, t float64) {
	if a.spiking {
		a.spikeUntil = math.Max(a.spikeUntil, t+seconds)
	} else {
		a.spiking = true
		a.spikeUntil = t + seconds
	}
}

func inCone(mouse, head point, f facing) bool {
	dx, dy := mouse.x-head.x, mouse.y-head.y
	distance := math.Hypot(dx, dy)
	if distance >= attRange || distance <= 1 {
		return false
	}
	if f == faceFront {
		return dy > -160 // 正面:面前與上方都算,腳底下不算
	}
	forward := 1.0
	if f == faceLeft {
		forward = -1
	}
	angle := math.Acos(dx*forward/distance) * 180 / math.Pi
	return angle <= attConeHalfAngle
}

// 眨眼節奏:三角分布抽間隔——大多 3.5 秒左右、偶爾連眨、偶爾放空,絕不等間隔。閉眼約 0.15 秒。
type blinker struct{ next float64 }

func (b *blinker) isClosed(t float64) bool {
	if b.next == 0 {
		b.next = t + blinkSample()
	}
	if t < b.next {
		return false
	}
	if t < b.next+0.15 {
		return true
	}
	b.next = t + blinkSample()
	return false
}

func blinkSample() float64 {
	const lo, mode, hi = 1.2, 3.5, 8.0
	u, c := rand.Float64(), (mode-lo)/(hi-lo)
	if u < c {
		return lo + math.Sqrt(u*(hi-lo)*(mode-lo))
	}
	return hi - math.Sqrt((1-u)*(hi-lo)*(hi-mode))
}

func randIn(lo, hi float64) float64 { return lo + rand.Float64()*(hi-lo) }
