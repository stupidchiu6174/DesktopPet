//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pSetTimer                      = user32.NewProc("SetTimer")
	pKillTimer                     = user32.NewProc("KillTimer")
	pUpdateLayeredWindow           = user32.NewProc("UpdateLayeredWindow")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pGetLastInputInfo              = user32.NewProc("GetLastInputInfo")
	pSetCapture                    = user32.NewProc("SetCapture")
	pReleaseCapture                = user32.NewProc("ReleaseCapture")
	pCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	pAppendMenuW                   = user32.NewProc("AppendMenuW")
	pTrackPopupMenu                = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                   = user32.NewProc("DestroyMenu")
	pSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	pEnumWindows                   = user32.NewProc("EnumWindows")
	pIsWindowVisible               = user32.NewProc("IsWindowVisible")
	pIsIconic                      = user32.NewProc("IsIconic")
	pGetWindowLongPtrW             = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	pGetClassNameW                 = user32.NewProc("GetClassNameW")
	pGetWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	pGetWindowRect                 = user32.NewProc("GetWindowRect")
	pMonitorFromPoint              = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pCreateIconIndirect            = user32.NewProc("CreateIconIndirect")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pDrawTextW                     = user32.NewProc("DrawTextW")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pRegisterWindowMessageW        = user32.NewProc("RegisterWindowMessageW")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateFontW        = gdi32.NewProc("CreateFontW")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pSetTextColor       = gdi32.NewProc("SetTextColor")

	pCreateBitmap  = gdi32.NewProc("CreateBitmap")
	pGdiFlush      = gdi32.NewProc("GdiFlush")
	pGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	pDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGetTickCount64   = kernel32.NewProc("GetTickCount64")
	pCreateMutexW     = kernel32.NewProc("CreateMutexW")
)

const (
	wsPopup           = 0x80000000
	wsChild           = 0x40000000
	wsExLayered       = 0x00080000
	wsExTopmost       = 0x00000008
	wsExToolWindow    = 0x00000080
	wsExNoActivate    = 0x08000000
	wsExTransparent   = 0x00000020
	wsExAppWindow     = 0x00040000
	gwlExStyle        = -20
	gwlStyle          = -16
	csDblClks         = 0x0008
	swShowNoActivate  = 4
	swHide            = 0
	ulwAlpha          = 2
	acSrcOver         = 0
	acSrcAlpha        = 1
	idcArrow          = 32512
	monitorNearest    = 2
	tpmReturnCmd      = 0x0100
	tpmRightButton    = 0x0002
	mfString          = 0x0000
	mfPopup           = 0x0010
	mfSeparator       = 0x0800
	mfChecked         = 0x0008
	dtCenter          = 0x0001
	dtSingleLine      = 0x0020
	dtVCenter         = 0x0004
	dtCalcRect        = 0x0400
	dtNoPrefix        = 0x0800
	logPixelsX        = 88
	dwmaCloaked       = 14
	dwmaExtendedFrame = 9
	swpNoMove         = 0x0002
	swpNoSize         = 0x0001
	swpNoActivate     = 0x0010
	hwndTopmost       = ^uintptr(0) // (HWND)-1

	wmDestroy        = 0x0002
	wmTimer          = 0x0113
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmLButtonDblClk  = 0x0203
	wmRButtonUp      = 0x0205
	wmMouseActivate  = 0x0021
	wmPowerBroadcast = 0x0218
	wmSetCursor      = 0x0020
	wmCaptureChanged = 0x0215
	wmApp            = 0x8000
	wmTray           = wmApp + 1
	maNoActivate     = 3
	pbtResumeAuto    = 0x12
	pbtResumeSuspend = 0x7

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 1
	nifIcon    = 2
	nifTip     = 4
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      winPoint
	private uint32
}

type winPoint struct{ x, y int32 }

type winRect struct{ left, top, right, bottom int32 }

type winSize struct{ cx, cy int32 }

type monitorInfo struct {
	cbSize  uint32
	monitor winRect
	work    winRect
	flags   uint32
}

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type blendFunction struct {
	op, flags, sourceConstantAlpha, alphaFormat byte
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         windows.GUID
	hBalloonIcon     uintptr
}

func utf16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func ptr[T any](v *T) uintptr { return uintptr(unsafe.Pointer(v)) }

// GetWindowLongPtr 的索引是負數,要照號延伸成 uintptr
func gwl(i int32) uintptr { return uintptr(i) }

// 32 位元、由上到下的 DIB(BGRA,預乘 alpha)
func newDIB(hdc uintptr, w, h int) (bitmap uintptr, bits []byte) {
	bi := bitmapInfoHeader{biWidth: int32(w), biHeight: -int32(h), biPlanes: 1, biBitCount: 32}
	bi.biSize = uint32(unsafe.Sizeof(bi))
	var p unsafe.Pointer
	bitmap, _, _ = pCreateDIBSection.Call(hdc, ptr(&bi), 0, uintptr(unsafe.Pointer(&p)), 0, 0)
	if bitmap == 0 || p == nil {
		return 0, nil
	}
	return bitmap, unsafe.Slice((*byte)(p), w*h*4)
}
