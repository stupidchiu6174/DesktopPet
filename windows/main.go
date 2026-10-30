//go:build windows

// DesktopPet 的 Windows 版:照 macOS 版(Sources/DesktopPet)移植的透明桌寵。
// 純 Win32(分層視窗 + UpdateLayeredWindow 逐像素 alpha),沒有 cgo,可以從 macOS / Linux 交叉編譯。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"
	"unsafe"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

func init() { runtime.LockOSThread() }

type settings struct {
	Outfit string `json:"outfit"`
}

type app struct {
	hwnd, host uintptr
	k          float64 // DPI 縮放:每一點幾個像素
	began      time.Time
	sprites    *sprites
	canvas     *canvas
	pet        *pet
	settings   settings
	visible    bool
	rate       float64

	memDC, dib, font uintptr
	bits             []byte
	winLeft, winTop  int32
	lastKey          string
	clickThrough     bool
	lastTopmost      float64

	pressed           point
	pressing, dragged bool
	doubleClick       bool
	taskbarCreated    uint32
	trayIcon          uintptr
	enumResult        []uintptr
	enumCallback      uintptr
}

var theApp *app

const (
	cmdOutfit = 1 // 1…3
	cmdSit    = 10
	cmdSleep  = 11
	cmdHide   = 20
	cmdShow   = 21
	cmdQuit   = 30
)

func main() {
	// 只開一隻
	if _, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16("DesktopPet.SingleInstance")))); err == windows.ERROR_ALREADY_EXISTS {
		return
	}
	if pSetProcessDpiAwarenessContext.Find() == nil {
		pSetProcessDpiAwarenessContext.Call(^uintptr(1)) // DPI_AWARENESS_CONTEXT_SYSTEM_AWARE (-2)
	} else {
		pSetProcessDPIAware.Call()
	}
	screenDC, _, _ := pGetDC.Call(0)
	dpi, _, _ := pGetDeviceCaps.Call(screenDC, logPixelsX)
	pReleaseDC.Call(0, screenDC)
	k := float64(dpi) / 96
	if k <= 0 {
		k = 1
	}

	a := &app{k: k, began: time.Now(), sprites: newSprites(k), canvas: newCanvas(k)}
	theApp = a
	a.pet = &pet{app: a}
	a.loadSettings()
	if o, ok := outfitNamed(a.settings.Outfit); ok {
		a.pet.outfit = o
	}
	if a.sprites.image(a.pet.outfit, stand, 0) == nil {
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16("找不到桌寵素材。"))), uintptr(unsafe.Pointer(utf16("DesktopPet"))), 0x10)
		return
	}
	if err := a.createWindows(); err != nil {
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16(err.Error()))), uintptr(unsafe.Pointer(utf16("DesktopPet"))), 0x10)
		return
	}
	a.addTrayIcon()
	a.show()

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(ptr(&m), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(ptr(&m))
		pDispatchMessageW.Call(ptr(&m))
	}
	a.removeTrayIcon()
}

func (a *app) createWindows() error {
	hinst, _, _ := pGetModuleHandleW.Call(0)
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)
	proc := windows.NewCallback(wndProc)
	for _, name := range []string{"DesktopPetWindow", "DesktopPetHost"} {
		wc := wndClassEx{style: csDblClks, lpfnWndProc: proc, hInstance: hinst, hCursor: cursor, lpszClassName: utf16(name)}
		wc.cbSize = uint32(unsafe.Sizeof(wc))
		if r, _, err := pRegisterClassExW.Call(ptr(&wc)); r == 0 {
			return fmt.Errorf("RegisterClassEx: %v", err)
		}
	}
	W, H := a.canvas.size()
	a.hwnd, _, _ = pCreateWindowExW.Call(wsExLayered|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(utf16("DesktopPetWindow"))), uintptr(unsafe.Pointer(utf16("DesktopPet"))),
		wsPopup, 0, 0, uintptr(W), uintptr(H), 0, 0, hinst, 0)
	// 看不見的視窗:收工作列圖示的訊息、當右鍵選單的主人
	a.host, _, _ = pCreateWindowExW.Call(wsExToolWindow,
		uintptr(unsafe.Pointer(utf16("DesktopPetHost"))), uintptr(unsafe.Pointer(utf16("DesktopPet"))),
		wsPopup, 0, 0, 0, 0, 0, 0, hinst, 0)
	if a.hwnd == 0 || a.host == 0 {
		return fmt.Errorf("CreateWindowEx failed")
	}
	a.memDC, _, _ = pCreateCompatibleDC.Call(0)
	a.dib, a.bits = newDIB(a.memDC, W, H)
	if a.dib == 0 {
		return fmt.Errorf("CreateDIBSection failed")
	}
	pSelectObject.Call(a.memDC, a.dib)
	a.font, _, _ = pCreateFontW.Call(uintptr(int32(-math.Round(13*a.k))), 0, 0, 0, 500, 0, 0, 0, 1, 0, 0, 4, 0,
		uintptr(unsafe.Pointer(utf16("Microsoft JhengHei UI"))))
	pSelectObject.Call(a.memDC, a.font)
	pSetBkMode.Call(a.memDC, 1) // TRANSPARENT
	pSetTextColor.Call(a.memDC, 0x262626)
	a.taskbarCreated = registerWindowMessage("TaskbarCreated")
	a.enumCallback = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		a.enumResult = append(a.enumResult, hwnd)
		return 1
	})
	return nil
}

func registerWindowMessage(name string) uint32 {
	r, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16(name))))
	return uint32(r)
}

// MARK: 平台:時間、滑鼠、閒置、螢幕、視窗邊沿

func (a *app) now() float64 { return time.Since(a.began).Seconds() }

// 滑鼠位置(點,y 朝上)
func (a *app) mouse() point {
	var p winPoint
	pGetCursorPos.Call(ptr(&p))
	return point{float64(p.x) / a.k, -float64(p.y) / a.k}
}

func (a *app) idleSeconds() float64 {
	li := lastInputInfo{cbSize: 8}
	pGetLastInputInfo.Call(ptr(&li))
	tick, _, _ := pGetTickCount64.Call()
	return float64(uint32(tick)-li.dwTime) / 1000
}

func (a *app) monitor(at point) monitorInfo {
	px, py := int32(at.x*a.k), int32(-at.y*a.k)
	h, _, _ := pMonitorFromPoint.Call(uintptr(uint32(px))|uintptr(uint32(py))<<32, monitorNearest)
	mi := monitorInfo{}
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	pGetMonitorInfoW.Call(h, ptr(&mi))
	return mi
}

// 這個點所在螢幕扣掉工作列的區域(點,y 朝上)
func (a *app) workArea(at point) rect {
	w := a.monitor(at).work
	return rect{float64(w.left) / a.k, -float64(w.bottom) / a.k, float64(w.right) / a.k, -float64(w.top) / a.k}
}

// 不當落腳處的系統視窗
var ignoredClasses = []string{"Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd",
	"Windows.UI.Core.CoreWindow", "NotifyIconOverflowWindow", "TopLevelWindowForOverflowXamlIsland"}

const (
	ledgeMinWidth, ledgeMinHeight = 200.0, 100.0
	ledgeMaxSources               = 6
	ledgeMinSegment               = 150.0
	// 太高的上緣不去(她的頭會撞到螢幕頂)
	ledgeHeadroom = 190.0
)

// 這個螢幕上所有能站的地方(第 0 個永遠是地面)。
// EnumWindows 天生由前到後排序,每個視窗的上緣用「在它前面的視窗」裁切,只有看得見的區段才算。
func (a *app) ledges(at point) []ledge {
	visible := a.workArea(at)
	result := []ledge{groundLedge(visible)}

	a.enumResult = a.enumResult[:0]
	pEnumWindows.Call(a.enumCallback, 0)

	type win struct {
		hwnd   uintptr
		r      winRect // 像素,y 朝下
		source bool
	}
	var wins []win
	sources := 0
	var class [64]uint16
	for _, h := range a.enumResult {
		if len(wins) >= 24 {
			break
		}
		if h == a.hwnd || h == a.host {
			continue
		}
		if v, _, _ := pIsWindowVisible.Call(h); v == 0 {
			continue
		}
		if v, _, _ := pIsIconic.Call(h); v != 0 {
			continue
		}
		var cloaked uint32
		if pDwmGetWindowAttribute.Call(h, dwmaCloaked, ptr(&cloaked), 4); cloaked != 0 {
			continue
		}
		style, _, _ := pGetWindowLongPtrW.Call(h, gwl(gwlStyle))
		ex, _, _ := pGetWindowLongPtrW.Call(h, gwl(gwlExStyle))
		if style&wsChild != 0 || ex&wsExTransparent != 0 || (ex&wsExToolWindow != 0 && ex&wsExAppWindow == 0) {
			continue
		}
		if n, _, _ := pGetWindowTextLengthW.Call(h); n == 0 {
			continue
		}
		n, _, _ := pGetClassNameW.Call(h, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if slices.Contains(ignoredClasses, windows.UTF16ToString(class[:n])) {
			continue
		}
		// 視窗看得到的外框(不含 Windows 10/11 看不見的縮放邊)
		var r winRect
		if hr, _, _ := pDwmGetWindowAttribute.Call(h, dwmaExtendedFrame, ptr(&r), unsafe.Sizeof(r)); hr != 0 {
			pGetWindowRect.Call(h, ptr(&r))
		}
		w, hgt := float64(r.right-r.left)/a.k, float64(r.bottom-r.top)/a.k
		if w <= 0 || hgt <= 0 {
			continue
		}
		source := w >= ledgeMinWidth && hgt >= ledgeMinHeight && sources < ledgeMaxSources
		if source {
			sources++
		}
		wins = append(wins, win{h, r, source})
	}

	for i, w := range wins {
		if !w.source {
			continue
		}
		top := -float64(w.r.top) / a.k
		minX, maxX := float64(w.r.left)/a.k, float64(w.r.right)/a.k
		// 要在這個螢幕上、離地面有點高度、頭上還有空間
		if top <= visible.minY+40 || top+ledgeHeadroom >= visible.maxY || maxX <= visible.minX || minX >= visible.maxX {
			continue
		}
		segments := [][2]float64{{minX, maxX}}
		for _, front := range wins[:i] {
			if w.r.top > front.r.top-1 && w.r.top < front.r.bottom-1 {
				segments = subtract(segments, float64(front.r.left)/a.k, float64(front.r.right)/a.k)
			}
		}
		for s, seg := range segments {
			lo, hi := math.Max(seg[0], visible.minX)+30, math.Min(seg[1], visible.maxX)-30
			if hi-lo < ledgeMinSegment-60 {
				continue
			}
			result = append(result, ledge{id: fmt.Sprintf("%x:%d", w.hwnd, s), y: top, xMin: lo, xMax: hi, anchorX: minX})
		}
	}
	return result
}

// MARK: 開關

func (a *app) show() {
	if a.visible {
		return
	}
	a.visible = true
	a.pet.start(a.now())
	a.lastKey = ""
	a.render()
	pShowWindow.Call(a.hwnd, swShowNoActivate)
	a.rate = 0
	a.setRate(30)
}

func (a *app) hide() {
	if !a.visible {
		return
	}
	a.visible = false
	pKillTimer.Call(a.hwnd, 1)
	a.rate = 0
	pShowWindow.Call(a.hwnd, swHide)
}

// 動的時候 30fps、發呆 12fps(呼吸和眨眼在 12fps 仍然自然)
func (a *app) setRate(hz float64) {
	if hz == a.rate {
		return
	}
	a.rate = hz
	pSetTimer.Call(a.hwnd, 1, uintptr(1000/hz), 0)
}

func (a *app) tick() {
	t := a.now()
	a.pet.tick(t)
	a.render()
	a.updateClickThrough()
	// 偶爾重新宣告最上層(有些程式會把自己也設成最上層)
	if t-a.lastTopmost > 5 {
		a.lastTopmost = t
		pSetWindowPos.Call(a.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
	if a.pet.isActive() || a.pressing {
		a.setRate(30)
	} else {
		a.setRate(12)
	}
}

// MARK: 畫面

func (a *app) render() {
	p := a.pet
	sp := p.sprite()
	if sp == nil {
		return
	}
	// 坐在視窗上緣時往下沉一點,像坐在牆頭上
	sink := 0.0
	if p.pose == sit && p.ledgeID != "ground" {
		sink = 6
	}
	W, H := a.canvas.size()
	left := int32(math.Round((p.feet.x - winW/2) * a.k))
	top := int32(math.Round(-(p.feet.y-sink)*a.k)) - int32(H)

	f := frameState{sprite: sp, facingLeft: p.facingLeft, bob: p.bob, tilt: p.tilt, squash: p.squash, poof: p.poof}
	key := fmt.Sprintf("%p %v %.1f %.2f %.4f %.3f %s", sp, p.facingLeft, p.bob, p.tilt, p.squash, p.poof, p.bubble)
	if key == a.lastKey && left == a.winLeft && top == a.winTop {
		return // 沒變就不重畫、不搬視窗
	}
	var text []uint16
	var textRect winRect
	if p.bubble != "" {
		text, _ = windows.UTF16FromString(p.bubble)
		pDrawTextW.Call(a.memDC, uintptr(unsafe.Pointer(&text[0])), ^uintptr(0), ptr(&textRect), dtCalcRect|dtSingleLine|dtNoPrefix)
		f.textW, f.textH = float64(textRect.right)/a.k, float64(textRect.bottom)/a.k
	}
	if key != a.lastKey {
		a.lastKey = key
		tr, hasText := a.canvas.draw(f)
		pGdiFlush.Call()
		src := a.canvas.img.Pix
		for i := 0; i < len(src); i += 4 {
			a.bits[i], a.bits[i+1], a.bits[i+2], a.bits[i+3] = src[i+2], src[i+1], src[i], src[i+3]
		}
		if hasText {
			r := winRect{int32(tr.Min.X), int32(tr.Min.Y), int32(tr.Max.X), int32(tr.Max.Y)}
			pDrawTextW.Call(a.memDC, uintptr(unsafe.Pointer(&text[0])), ^uintptr(0), ptr(&r), dtCenter|dtVCenter|dtSingleLine|dtNoPrefix)
			pGdiFlush.Call()
			// GDI 寫過的像素 alpha 會變 0;泡泡裡本來就是不透明白底,補回 255
			for y := max(0, tr.Min.Y); y < min(H, tr.Max.Y); y++ {
				for x := max(0, tr.Min.X); x < min(W, tr.Max.X); x++ {
					a.bits[(y*W+x)*4+3] = 255
				}
			}
		}
	}
	a.winLeft, a.winTop = left, top
	pos := winPoint{left, top}
	size := winSize{int32(W), int32(H)}
	zero := winPoint{}
	blend := blendFunction{op: acSrcOver, sourceConstantAlpha: 255, alphaFormat: acSrcAlpha}
	pUpdateLayeredWindow.Call(a.hwnd, 0, ptr(&pos), ptr(&size), a.memDC, ptr(&zero), 0, ptr(&blend), ulwAlpha)
}

// 滑鼠在角色身上才接收點擊,其他地方穿透到下面的視窗
func (a *app) updateClickThrough() {
	through := !a.pressing && !a.hitsCharacter()
	if through == a.clickThrough {
		return
	}
	a.clickThrough = through
	idx := gwl(gwlExStyle)
	ex, _, _ := pGetWindowLongPtrW.Call(a.hwnd, idx)
	if through {
		ex |= wsExTransparent
	} else {
		ex &^= wsExTransparent
	}
	pSetWindowLongPtrW.Call(a.hwnd, idx, ex)
}

func (a *app) hitsCharacter() bool {
	var c winPoint
	pGetCursorPos.Call(ptr(&c))
	W, H := a.canvas.size()
	x, y := int(c.x-a.winLeft), int(c.y-a.winTop)
	if x < 0 || y < 0 || x >= W || y >= H {
		return false
	}
	return a.canvas.hit[y*W+x] > 38
}

// MARK: 視窗訊息

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	a := theApp
	if a == nil {
		r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
		return r
	}
	t := a.now()
	switch {
	case message == wmTimer && hwnd == a.hwnd:
		if a.visible {
			a.tick()
		}
		return 0
	case message == wmMouseActivate:
		return maNoActivate
	case message == wmLButtonDown || message == wmLButtonDblClk:
		a.pressed = a.mouse()
		a.pressing, a.dragged = true, false
		a.doubleClick = message == wmLButtonDblClk
		pSetCapture.Call(hwnd)
		return 0
	case message == wmMouseMove && hwnd == a.hwnd:
		if !a.pressing {
			return 0
		}
		m := a.mouse()
		if !a.dragged && math.Hypot(m.x-a.pressed.x, m.y-a.pressed.y) < 4 {
			return 0
		}
		if !a.dragged {
			a.dragged = true
			a.pet.grab(t)
			a.setRate(30)
		}
		a.pet.drag(m, t)
		a.render()
		return 0
	case message == wmLButtonUp:
		if !a.pressing {
			return 0
		}
		a.pressing = false
		pReleaseCapture.Call()
		switch {
		case a.dragged:
			a.pet.drop(t)
		case a.doubleClick:
			a.pet.nextOutfitInCycle()
		default:
			a.pet.click(t)
		}
		a.doubleClick = false
		return 0
	case message == wmCaptureChanged:
		// 拖到一半被搶走滑鼠(例如跳出系統對話框):當作放開
		if a.pressing {
			a.pressing = false
			if a.dragged {
				a.pet.drop(t)
			}
		}
		return 0
	case message == wmRButtonUp && hwnd == a.hwnd:
		a.petMenu()
		return 0
	case message == wmTray:
		switch uint32(lParam) {
		case wmLButtonUp:
			if a.visible {
				a.hide()
			} else {
				a.show()
			}
		case wmRButtonUp:
			a.trayMenu()
		}
		return 0
	case message == a.taskbarCreated && message != 0:
		// 檔案總管重開:工作列圖示要重新加
		a.addTrayIcon()
		return 0
	case message == wmPowerBroadcast:
		// 睡醒的那一刻視窗狀態還不穩:靜置兩秒再抓視窗
		if wParam == pbtResumeAuto || wParam == pbtResumeSuspend {
			a.pet.wakeSettleTill = t + 2
		}
		return 1
	case message == wmDestroy && hwnd == a.host:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

// MARK: 選單

func appendMenu(menu uintptr, flags, id uintptr, title string) {
	pAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(utf16(title))))
}

func (a *app) popup(menu uintptr) {
	var c winPoint
	pGetCursorPos.Call(ptr(&c))
	pSetForegroundWindow.Call(a.host)
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton, uintptr(c.x), uintptr(c.y), 0, a.host, 0)
	pPostMessageW.Call(a.host, 0, 0, 0) // WM_NULL:讓選單正常關掉
	pDestroyMenu.Call(menu)
	a.command(int(cmd))
}

func (a *app) petMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	sub, _, _ := pCreatePopupMenu.Call()
	for _, o := range outfits {
		flags := uintptr(mfString)
		if o == a.pet.outfit {
			flags |= mfChecked
		}
		appendMenu(sub, flags, uintptr(cmdOutfit+int(o)), o.title())
	}
	appendMenu(menu, mfPopup, sub, "換衣服")
	appendMenu(menu, mfString, cmdSit, "坐下休息")
	appendMenu(menu, mfString, cmdSleep, "去睡覺")
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, cmdHide, "收起桌寵")
	appendMenu(menu, mfString, cmdQuit, "結束")
	a.popup(menu)
}

func (a *app) trayMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if a.visible {
		appendMenu(menu, mfString, cmdHide, "收起桌寵")
	} else {
		appendMenu(menu, mfString, cmdShow, "叫出桌寵")
	}
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, cmdQuit, "結束")
	a.popup(menu)
}

func (a *app) command(cmd int) {
	t := a.now()
	switch {
	case cmd >= cmdOutfit && cmd < cmdOutfit+len(outfits):
		a.pet.setOutfit(outfits[cmd-cmdOutfit])
	case cmd == cmdSit:
		a.pet.enterSitting(t)
	case cmd == cmdSleep:
		a.pet.life = life{kind: lifeSleeping, manual: true}
		a.pet.say("晚安～", 2)
	case cmd == cmdHide:
		a.hide()
	case cmd == cmdShow:
		a.show()
	case cmd == cmdQuit:
		pShowWindow.Call(a.hwnd, swHide)
		pDestroyWindow.Call(a.hwnd)
		pDestroyWindow.Call(a.host)
	}
}

// MARK: 工作列圖示(收起來之後從這裡叫回來)

func (a *app) addTrayIcon() {
	if a.trayIcon == 0 {
		a.trayIcon = a.makeIcon()
	}
	nid := notifyIconData{hWnd: a.host, uID: 1, uFlags: nifMessage | nifIcon | nifTip, uCallbackMessage: wmTray, hIcon: a.trayIcon}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	tip, _ := windows.UTF16FromString("DesktopPet 桌寵(點一下收起 / 叫出)")
	copy(nid.szTip[:len(nid.szTip)-1], tip)
	pShellNotifyIconW.Call(nimAdd, ptr(&nid))
}

func (a *app) removeTrayIcon() {
	nid := notifyIconData{hWnd: a.host, uID: 1}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	pShellNotifyIconW.Call(nimDelete, ptr(&nid))
}

// 用站姿的頭當圖示
func (a *app) makeIcon() uintptr {
	sp := a.sprites.image(a.pet.outfit, stand, 0)
	if sp == nil {
		return 0
	}
	const n = 32
	b := sp.img.Rect
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y, b.Min.X+(b.Dx()-side)/2+side, b.Min.Y+side)
	icon := image.NewRGBA(image.Rect(0, 0, n, n))
	xdraw.CatmullRom.Scale(icon, icon.Rect, sp.img, crop, xdraw.Src, nil)

	color, bits := newDIB(0, n, n)
	if color == 0 {
		return 0
	}
	// 圖示用的是沒預乘的 alpha
	for i := 0; i < n*n*4; i += 4 {
		r, g, bl, al := icon.Pix[i], icon.Pix[i+1], icon.Pix[i+2], icon.Pix[i+3]
		if al > 0 {
			r, g, bl = uint8(int(r)*255/int(al)), uint8(int(g)*255/int(al)), uint8(int(bl)*255/int(al))
		}
		bits[i], bits[i+1], bits[i+2], bits[i+3] = bl, g, r, al
	}
	mask := make([]byte, n*n/8)
	maskBmp, _, _ := pCreateBitmap.Call(n, n, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	ii := iconInfo{fIcon: 1, hbmMask: maskBmp, hbmColor: color}
	h, _, _ := pCreateIconIndirect.Call(ptr(&ii))
	pDeleteObject.Call(maskBmp)
	pDeleteObject.Call(color)
	return h
}

// MARK: 設定(%APPDATA%\DesktopPet\settings.json)

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "DesktopPet", "settings.json")
}

func (a *app) loadSettings() {
	if data, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(data, &a.settings)
	}
}

func (a *app) saveSettings() {
	path := settingsPath()
	if path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(a.settings, "", "  ")
	os.WriteFile(path, data, 0o644)
}
