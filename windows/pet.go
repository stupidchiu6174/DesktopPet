//go:build windows

package main

import (
	"math"
	"math/rand"
)

// 桌寵的生活邏輯,照 macOS 版 PetController(Sources/DesktopPet/PetController.swift)移植。
//
// 她的世界 = 地面(工作列上緣)+ 一般視窗看得到的上緣。
// 站著發呆 20–60 秒 → 七成走去某處坐 30–120 秒、三成散步(散步時 35% 跳到別的視窗上)。
// 腳下的視窗被拖動她會跟著移動,被拖走或關掉她就掉下去,途中可以被別的視窗接住。
// 電腦放著五分鐘她就睡著、你回來會醒。

type lifeKind int

const (
	lifeStanding lifeKind = iota
	lifeMoving
	lifeJumping
	lifeSitting
	lifeFalling
	lifeHeld
	lifeSleeping
	lifeDancing
	lifeReacting
)

type life struct {
	kind     lifeKind
	until    float64 // standing / sitting / reacting
	target   float64 // moving
	sitAfter bool    // moving / jumping
	from, to point   // jumping
	start    float64
	duration float64
	ledge    string
	velocity float64 // falling
	pose     pose    // reacting
	// sleeping:從選單叫她睡的,要點她才醒;放著電腦睡著的,你一回來就醒
	manual bool
}

type pet struct {
	app *app

	outfit     outfit
	pose       pose
	facingLeft bool
	bob, tilt  float64
	squash     float64
	walkFrame  int
	bubble     string
	bubbleTill float64
	poof       float64
	poofTarget float64
	poofSwapAt float64
	nextOutfit outfit

	life      life
	attention attention
	blink     blinker
	// 腳底中央的位置
	feet           point
	ledges         []ledge
	ledgeID        string
	lastLedgePoll  float64
	wakeSettleTill float64
	last           float64
}

func (p *pet) current() *ledge {
	for i := range p.ledges {
		if p.ledges[i].id == p.ledgeID {
			return &p.ledges[i]
		}
	}
	return nil
}

func (p *pet) start(t float64) {
	work := p.app.workArea(point{}) // 主螢幕
	p.ledges = []ledge{groundLedge(work)}
	p.ledgeID = "ground"
	p.feet = point{work.maxX - 220, p.ledges[0].y}
	p.squash = 1
	p.enterStanding(t)
	p.last = t
}

func (p *pet) sprite() *sprite { return p.app.sprites.image(p.outfit, p.pose, p.walkFrame) }

func (p *pet) isActive() bool {
	if p.attention.spiking || p.poof != p.poofTarget || p.poofSwapAt > 0 {
		return true
	}
	switch p.life.kind {
	case lifeMoving, lifeJumping, lifeFalling, lifeHeld, lifeDancing, lifeReacting:
		return true
	}
	return false
}

// 重抓視窗邊沿。站在視窗上時每秒 10 次(要跟得上被拖的視窗),在地上每秒 1 次就夠
func (p *pet) pollLedges(t float64) {
	if t < p.wakeSettleTill {
		return
	}
	interval := 1.0
	switch {
	case p.life.kind == lifeFalling || p.life.kind == lifeHeld || p.life.kind == lifeJumping:
		interval = 0.1
	case p.ledgeID != "ground":
		interval = 0.1
	}
	if t-p.lastLedgePoll < interval {
		return
	}
	p.lastLedgePoll = t
	var old *ledge
	if c := p.current(); c != nil {
		o := *c
		old = &o
	}
	p.ledges = p.app.ledges(p.feet)

	switch p.life.kind {
	case lifeJumping, lifeFalling, lifeHeld:
		return
	}
	l := p.current()
	if l == nil {
		// 腳下的視窗被關掉 / 移走了:掉
		if old != nil && !old.isGround() {
			p.startFalling()
		}
		p.ledgeID = "ground"
		return
	}
	// 視窗被拖著走:她騎在上面,水平跟著移動
	if old != nil && old.id == l.id {
		p.feet.x += l.anchorX - old.anchorX
	}
	// 腳下已經沒有這段邊沿 → 掉,不是滑到邊緣
	if p.feet.x < l.xMin-20 || p.feet.x > l.xMax+20 {
		p.startFalling()
		return
	}
	p.feet.y = l.y
}

func (p *pet) tick(t float64) {
	dt := math.Min(t-p.last, 0.15)
	p.last = t
	if p.bubble != "" && t > p.bubbleTill {
		p.bubble = ""
	}
	// 換衣服:縮小變淡 → 換 → 彈回來
	if p.poofSwapAt > 0 && t >= p.poofSwapAt {
		p.poofSwapAt = 0
		p.outfit = p.nextOutfit
		p.poofTarget = 0
		p.say(p.outfit.greeting(), 2.5)
	}
	p.poof += (p.poofTarget - p.poof) * math.Min(1, 18*dt)
	if math.Abs(p.poof-p.poofTarget) < 0.01 {
		p.poof = p.poofTarget
	}

	p.pollLedges(t)
	p.lifeEvents(t)

	// 注意力:頭大約在腳上方 130 點
	f := faceFront
	if p.pose == walk {
		f = faceRight
		if p.facingLeft {
			f = faceLeft
		}
	}
	if p.attention.update(t, dt, p.app.mouse(), point{p.feet.x, p.feet.y + 130}, f) {
		p.afterSpike(t)
	}

	p.step(t, dt)

	// 站著時眨眼
	if p.pose == stand && p.blink.isClosed(t) {
		p.pose = blink
	}

	// 呼吸 / 落地壓扁慢慢回彈
	breathe := 0.008
	if p.life.kind == lifeSleeping {
		breathe = 0.02
	}
	target := 1 + breathe*math.Sin(t*2.2)
	p.squash += (target - p.squash) * math.Min(1, 10*dt)
}

// 你離開太久就睡、回來就醒
func (p *pet) lifeEvents(t float64) {
	switch p.life.kind {
	case lifeHeld, lifeFalling, lifeJumping:
		return
	}
	idle := p.app.idleSeconds()
	if p.life.kind == lifeSleeping {
		if !p.life.manual && idle < 1.5 {
			p.life = life{kind: lifeReacting, pose: surprised, until: t + 1.2}
			p.say("…啊,你回來了", 2.5)
		}
		return
	}
	if idle > 300 {
		p.life = life{kind: lifeSleeping}
	}
}

func (p *pet) step(t, dt float64) {
	spiking := p.attention.spiking
	p.walkFrame = 0
	l := &p.life

	switch l.kind {
	case lifeStanding:
		p.pose, p.bob, p.tilt = stand, 0, 0
		if spiking {
			p.faceMouse()
			return
		}
		// 被冷落時,發呆的時間過得比較快(兩倍速)
		if p.attention.value < attNeglected {
			l.until -= dt
		}
		// 七成找地方坐(不換邊),三成純散步(可能跳到別的視窗)
		if t >= l.until {
			p.decideNewSpot(t, rand.Float64() < 0.7)
		}

	case lifeMoving:
		if spiking {
			// 轉過來看你的時候停下腳步,不原地踏步
			p.pose, p.bob, p.tilt = stand, 0, 0
			p.faceMouse()
			return
		}
		dx := l.target - p.feet.x
		speed := 45.0
		if p.outfit == kimono {
			speed = 32
		}
		if math.Abs(dx) < 2 {
			if l.sitAfter {
				p.enterSitting(t)
			} else {
				p.enterStanding(t)
			}
			return
		}
		p.facingLeft = dx < 0
		p.feet.x += math.Max(-speed*dt, math.Min(speed*dt, dx))
		if c := p.current(); c != nil {
			p.feet.y = c.y
		}
		p.pose = walk
		if p.app.sprites.walkFrames(p.outfit) > 0 {
			p.walkFrame = int(t * 8 * speed / 45)
			p.bob = math.Abs(math.Sin(t*math.Pi*4)) * 1.5
			p.tilt = 0
		} else {
			p.bob = math.Abs(math.Sin(t*math.Pi*4)) * 4
			p.tilt = math.Sin(t*math.Pi*4) * 2
		}

	case lifeJumping:
		pr := math.Min(1, (t-l.start)/l.duration)
		arc := math.Max(70, math.Abs(l.to.y-l.from.y)*0.35)
		p.feet = point{l.from.x + (l.to.x-l.from.x)*pr, l.from.y + (l.to.y-l.from.y)*pr + arc*4*pr*(1-pr)}
		p.pose, p.bob, p.tilt = happy, 0, 0
		if pr >= 1 {
			p.feet = l.to
			p.ledgeID = l.ledge
			p.squash = 0.9
			if l.sitAfter {
				p.enterSitting(t)
			} else {
				p.enterStanding(t)
			}
		}

	case lifeSitting:
		p.pose, p.bob, p.tilt = sit, 0, 0
		if spiking {
			p.faceMouse()
			return
		}
		if t > l.until {
			p.enterStanding(t)
		}

	case lifeFalling:
		v := l.velocity - 2200*dt
		previous := p.feet.y
		p.feet.y += v * dt
		p.pose, p.bob, p.tilt = surprised, 0, 0
		// 這一格的掉落路徑上、接得住她的最高邊沿(地面永遠接得住)
		var catcher *ledge
		for i := range p.ledges {
			c := &p.ledges[i]
			if (c.isGround() || (p.feet.x >= c.xMin && p.feet.x <= c.xMax)) && c.y <= previous+0.5 && c.y >= p.feet.y {
				if catcher == nil || c.y > catcher.y {
					catcher = c
				}
			}
		}
		if catcher == nil {
			l.velocity = v
			return
		}
		p.feet.y = catcher.y
		if catcher.isGround() {
			p.feet.x = catcher.clamp(p.feet.x)
		}
		p.ledgeID = catcher.id
		impact := -v
		p.squash = 0.92
		if impact > 600 {
			p.squash = 0.8
		}
		if impact > 900 {
			p.life = life{kind: lifeReacting, pose: surprised, until: t + 1.0}
			p.say([]string{"哎唷!", "好痛…"}[rand.Intn(2)], 2.5)
		} else {
			p.enterStanding(t)
		}

	case lifeHeld:
		p.pose = held
		p.tilt = math.Sin(t*3) * 6
		p.bob = 0

	case lifeSleeping:
		p.pose, p.bob, p.tilt = sleep, 0, 0
		if p.bubble == "" && int(t)%7 == 0 {
			p.say("Zz…", 2)
		}

	case lifeDancing:
		p.pose = dance
		p.bob = math.Abs(math.Sin(t*math.Pi*4)) * 8
		p.tilt = math.Sin(t*math.Pi*2) * 6

	case lifeReacting:
		p.pose, p.tilt, p.bob = l.pose, 0, 0
		if l.pose == happy {
			p.bob = math.Abs(math.Sin(t*math.Pi*3)) * 12
		}
		if t > l.until {
			p.enterStanding(t)
		}
	}
}

func (p *pet) enterStanding(t float64) { p.life = life{kind: lifeStanding, until: t + randIn(20, 60)} }

func (p *pet) enterSitting(t float64) { p.life = life{kind: lifeSitting, until: t + randIn(30, 120)} }

// 換個地方:要坐就在住的這條邊上找位子;純散步才可能跳到別的視窗
func (p *pet) decideNewSpot(t float64, sitAfter bool) {
	l := p.current()
	if l == nil {
		if len(p.ledges) == 0 {
			return
		}
		l = &p.ledges[0]
	}
	var others []ledge
	for _, o := range p.ledges {
		if o.id != l.id {
			others = append(others, o)
		}
	}
	if !sitAfter && len(others) > 0 && rand.Float64() < 0.35 {
		p.jump(others[rand.Intn(len(others))], rand.Float64() < 0.4, t)
		return
	}
	x := l.clamp(p.feet.x + randIn(-420, 420))
	p.life = life{kind: lifeMoving, target: x, sitAfter: sitAfter}
}

func (p *pet) jump(target ledge, sitAfter bool, t float64) {
	to := point{target.clamp(p.feet.x + randIn(-80, 80)), target.y}
	distance := math.Hypot(to.x-p.feet.x, to.y-p.feet.y)
	p.facingLeft = to.x < p.feet.x
	p.life = life{kind: lifeJumping, from: p.feet, to: to, start: t,
		duration: math.Min(1.3, math.Max(0.6, distance/700)), ledge: target.id, sitAfter: sitAfter}
}

// spike 結束:常常「假裝沒事」換個位置
func (p *pet) afterSpike(t float64) {
	if (p.life.kind == lifeStanding || p.life.kind == lifeSitting) && rand.Float64() < 0.6 {
		p.decideNewSpot(t, rand.Float64() < 0.5)
	}
}

func (p *pet) faceMouse() { p.facingLeft = p.app.mouse().x < p.feet.x }

func (p *pet) startFalling() { p.life = life{kind: lifeFalling} }

func (p *pet) say(text string, seconds float64) {
	p.bubble = text
	p.bubbleTill = p.app.now() + seconds
}

func (p *pet) setOutfit(o outfit) {
	if o == p.outfit || p.poofSwapAt > 0 {
		return
	}
	p.nextOutfit = o
	p.poofTarget = 1
	p.poofSwapAt = p.app.now() + 0.18
	p.app.settings.Outfit = o.name()
	p.app.saveSettings()
}

// MARK: 滑鼠

func (p *pet) grab(t float64) {
	p.life = life{kind: lifeHeld}
	p.attention.hold(2, t)
}

// 拎著頭:腳底跟在滑鼠下方
func (p *pet) drag(mouse point, t float64) {
	p.attention.hold(1, t)
	h := 150.0
	if sp := p.sprite(); sp != nil {
		h = sp.h
	}
	p.feet = point{mouse.x, mouse.y - h*0.85}
}

// 放開:從這個高度掉下去,被視窗或地面接住
func (p *pet) drop(t float64) {
	p.ledges = p.app.ledges(p.feet)
	p.lastLedgePoll = t
	p.startFalling()
}

func (p *pet) click(t float64) {
	if p.life.kind == lifeSleeping {
		p.life = life{kind: lifeReacting, pose: surprised, until: t + 1.2}
		p.say("嗯…?我沒有在睡喔", 2.5)
		return
	}
	// 被你點了:看你一下,開心一下
	p.attention.hold(3, t)
	switch p.life.kind {
	case lifeStanding, lifeMoving, lifeDancing, lifeReacting:
		p.life = life{kind: lifeReacting, pose: happy, until: t + 1.2}
	}
}

func (p *pet) nextOutfitInCycle() { p.setOutfit(outfits[(int(p.outfit)+1)%len(outfits)]) }
