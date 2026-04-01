package overlay

import (
	"fmt"
	"log"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	msimg32  = syscall.NewLazyDLL("msimg32.dll")

	pCreateWindowEx             = user32.NewProc("CreateWindowExW")
	pDefWindowProc              = user32.NewProc("DefWindowProcW")
	pRegisterClass              = user32.NewProc("RegisterClassExW")
	pShowWindow                 = user32.NewProc("ShowWindow")
	pGetSysMetrics              = user32.NewProc("GetSystemMetrics")
	pSetTimer                   = user32.NewProc("SetTimer")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pGetModuleHandle            = kernel32.NewProc("GetModuleHandleW")
	pPeekMessage                = user32.NewProc("PeekMessageW")
	pTranslateMessage           = user32.NewProc("TranslateMessage")
	pDispatchMessage            = user32.NewProc("DispatchMessageW")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pSetBkMode          = gdi32.NewProc("SetBkMode")
	pSetTextColor       = gdi32.NewProc("SetTextColor")
	pCreateFont         = gdi32.NewProc("CreateFontW")
	pDrawText           = user32.NewProc("DrawTextW")
	pUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	pGetDC              = user32.NewProc("GetDC")
	pReleaseDC          = user32.NewProc("ReleaseDC")
)

const (
	WS_EX_LAYERED    = 0x00080000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_NOACTIVATE = 0x08000000
	WS_POPUP         = 0x80000000
	SW_SHOW          = 5
	SW_HIDE          = 0
	SM_CXSCREEN      = 0
	SM_CYSCREEN      = 1
	WM_TIMER         = 0x0113
	TRANSPARENT      = 1
	DT_CENTER        = 0x01
	DT_VCENTER       = 0x04
	DT_SINGLELINE    = 0x20
	ULW_ALPHA        = 0x02
	AC_SRC_OVER      = 0x00
	AC_SRC_ALPHA     = 0x01

	pillW = 280
	pillH = 38
)

type POINT struct{ X, Y int32 }
type SIZE struct{ CX, CY int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}
type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type overlayState int

const (
	stateIdle overlayState = iota
	stateRecording
	stateTranscribing
	stateSuccess
	stateError
)

type Overlay struct {
	mu      sync.Mutex
	hwnd    uintptr
	status  string
	r, g, b byte
	state   overlayState
	visible bool
	hideAt  time.Time
	ready   chan struct{}

	// Animation state
	animTick  int
	startTime time.Time
}

var globalOverlay *Overlay

func New() *Overlay {
	o := &Overlay{
		r: 0x8B, g: 0x5C, b: 0xF6,
		ready:     make(chan struct{}),
		state:     stateIdle,
		startTime: time.Now(),
	}
	globalOverlay = o
	return o
}

func (o *Overlay) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Overlay recovered from panic: %v", r)
		}
	}()

	className, _ := syscall.UTF16PtrFromString("YappieOverlay")
	hInst, _, _ := pGetModuleHandle.Call(0)

	wc := WNDCLASSEX{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
		WndProc:   syscall.NewCallback(overlayWndProc),
		Instance:  hInst,
		ClassName: className,
	}
	pRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))

	screenW, _, _ := pGetSysMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := pGetSysMetrics.Call(SM_CYSCREEN)

	x := (int(screenW) - pillW) / 2
	y := int(screenH) - pillH - 60

	title, _ := syscall.UTF16PtrFromString("")
	hwnd, _, _ := pCreateWindowEx.Call(
		WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		WS_POPUP,
		uintptr(x), uintptr(y), pillW, pillH,
		0, 0, hInst, 0,
	)
	o.hwnd = hwnd

	// Animation timer — 30fps for smooth animations
	pSetTimer.Call(hwnd, 1, 33, 0)

	close(o.ready)

	var msg MSG
	for {
		ret, _, _ := pPeekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
		if ret != 0 {
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
		time.Sleep(8 * time.Millisecond)
	}
}

func overlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TIMER:
		if globalOverlay != nil {
			globalOverlay.onTick()
		}
	}
	ret, _, _ := pDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return ret
}

func (o *Overlay) onTick() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.visible {
		return
	}

	o.animTick++

	// Auto-hide check
	if !o.hideAt.IsZero() && time.Now().After(o.hideAt) {
		o.setIdleLocked()
		return
	}

	// Animate recording state (pulsing dot)
	if o.state == stateRecording || o.state == stateTranscribing {
		o.renderLocked(pillW, pillH)
	}
}

// ── Public API ──

func (o *Overlay) Show(status string, r, g, b byte, autoHideMs int) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.status = status
	o.r, o.g, o.b = r, g, b
	o.state = stateSuccess
	o.animTick = 0
	o.startTime = time.Now()
	if autoHideMs > 0 {
		o.hideAt = time.Now().Add(time.Duration(autoHideMs) * time.Millisecond)
	} else {
		o.hideAt = time.Time{}
	}
	o.visible = true
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) Hide() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.visible = false
	pShowWindow.Call(o.hwnd, SW_HIDE)
}

func (o *Overlay) ShowIdle() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.setIdleLocked()
}

func (o *Overlay) setIdleLocked() {
	o.status = ""
	o.r, o.g, o.b = 0x8B, 0x5C, 0xF6
	o.state = stateIdle
	o.visible = true
	o.hideAt = time.Time{}
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) ShowRecording() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.status = "Listening..."
	o.r, o.g, o.b = 0xEF, 0x44, 0x44
	o.state = stateRecording
	o.animTick = 0
	o.startTime = time.Now()
	o.hideAt = time.Time{}
	o.visible = true
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) ShowTranscribing() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.status = "Processing..."
	o.r, o.g, o.b = 0xD9, 0x8C, 0x2E
	o.state = stateTranscribing
	o.animTick = 0
	o.startTime = time.Now()
	o.hideAt = time.Time{}
	o.visible = true
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) ShowSuccess(words int) {
	msg := "Text injected"
	if words == 1 {
		msg = "1 word injected"
	} else if words > 1 {
		msg = fmt.Sprintf("%d words injected", words)
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	o.status = msg
	o.r, o.g, o.b = 0x34, 0xD3, 0x99
	o.state = stateSuccess
	o.animTick = 0
	o.startTime = time.Now()
	o.hideAt = time.Now().Add(2500 * time.Millisecond)
	o.visible = true
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) ShowError(msg string) {
	if len(msg) > 35 {
		msg = msg[:32] + "..."
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	o.status = msg
	o.r, o.g, o.b = 0xEF, 0x44, 0x44
	o.state = stateError
	o.animTick = 0
	o.startTime = time.Now()
	o.hideAt = time.Now().Add(3 * time.Second)
	o.visible = true
	o.renderLocked(pillW, pillH)
	pShowWindow.Call(o.hwnd, SW_SHOW)
}

func (o *Overlay) ShowReady(hotkey string) {
	o.Show("Hold "+hotkey+" to dictate", 0x8B, 0x5C, 0xF6, 3000)
}

func (o *Overlay) WaitReady() {
	<-o.ready
}

// ── Rendering ──

func (o *Overlay) renderLocked(w, h int) {
	screenDC, _, _ := pGetDC.Call(0)
	memDC, _, _ := pCreateCompatibleDC.Call(screenDC)

	bmi := BITMAPINFOHEADER{
		Size:     uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		Width:    int32(w),
		Height:   -int32(h),
		Planes:   1,
		BitCount: 32,
	}

	var bits uintptr
	hBmp, _, _ := pCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	pSelectObject.Call(memDC, hBmp)

	pixels := (*[1 << 25]byte)(unsafe.Pointer(bits))

	// Clear
	for i := 0; i < w*h*4; i++ {
		pixels[i] = 0
	}

	if o.state == stateIdle {
		// Idle: subtle breathing dot
		breathe := 0.7 + 0.3*math.Sin(float64(o.animTick)*0.05)
		alpha := byte(80 * breathe)
		drawCircle(pixels, w, h, w/2, h/2, 5, o.r, o.g, o.b, alpha)
	} else {
		// Active states: pill with glassmorphism effect

		// Background — dark glass
		drawRoundedRect(pixels, w, h, 0, 0, w, h, h/2, 15, 15, 22, 220)

		// Top highlight (glass effect)
		for y := 0; y < h/3; y++ {
			for x := 0; x < w; x++ {
				lx := float64(x)
				ly := float64(y)
				fw := float64(w)
				fh := float64(h)
				dist := roundedRectDist(lx, ly, fw, fh, float64(h/2))
				if dist < -1.0 {
					grad := 1.0 - float64(y)/float64(h/3)
					a := byte(12 * grad)
					blendPixel(pixels, w, x, y, 255, 255, 255, a)
				}
			}
		}

		// Subtle border
		drawRoundedRectBorder(pixels, w, h, 0, 0, w, h, h/2, 60, 60, 80, 35)

		// Status indicator
		switch o.state {
		case stateRecording:
			// Pulsing red dot
			pulse := 0.6 + 0.4*math.Sin(float64(o.animTick)*0.15)
			radius := 5.0 + 1.5*pulse
			alpha := byte(255 * (0.7 + 0.3*pulse))
			// Glow
			drawCircle(pixels, w, h, 20, h/2, int(radius+3), o.r, o.g, o.b, byte(float64(alpha)*0.25))
			drawCircle(pixels, w, h, 20, h/2, int(radius), o.r, o.g, o.b, alpha)

			// Recording timer
			elapsed := time.Since(o.startTime)
			secs := int(elapsed.Seconds())
			timerStr := fmt.Sprintf("%d:%02d", secs/60, secs%60)
			// Draw timer on the right side
			drawTextGDI(pixels, w, h, timerStr, memDC, w-60, 0, w-10, h, 0x99, 0x99, 0xAA)

		case stateTranscribing:
			// Spinning dots animation
			cx, cy := 20, h/2
			for i := 0; i < 3; i++ {
				angle := float64(o.animTick)*0.12 + float64(i)*2.1
				dx := int(math.Cos(angle) * 4)
				dy := int(math.Sin(angle) * 4)
				a := byte(180 + 75*math.Sin(angle))
				drawCircle(pixels, w, h, cx+dx, cy+dy, 2, o.r, o.g, o.b, a)
			}

		case stateSuccess:
			// Checkmark dot
			drawCircle(pixels, w, h, 20, h/2, 6, o.r, o.g, o.b, 255)
			// Simple checkmark using pixels
			drawCheck(pixels, w, 17, h/2-1, 255, 255, 255, 240)

		case stateError:
			// X dot
			drawCircle(pixels, w, h, 20, h/2, 6, o.r, o.g, o.b, 255)
			drawX(pixels, w, 20, h/2, 255, 255, 255, 240)
		}

		// Main text
		drawTextGDI(pixels, w, h, o.status, memDC, 36, 0, w-12, h, 0xE0, 0xE0, 0xE4)
	}

	// Update layered window
	ptSrc := POINT{0, 0}
	sz := SIZE{int32(w), int32(h)}

	screenW, _, _ := pGetSysMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := pGetSysMetrics.Call(SM_CYSCREEN)
	x := (int(screenW) - w) / 2
	y := int(screenH) - h - 60
	ptDst := POINT{int32(x), int32(y)}

	blend := BLENDFUNCTION{
		BlendOp:             AC_SRC_OVER,
		SourceConstantAlpha: 255,
		AlphaFormat:         AC_SRC_ALPHA,
	}

	pUpdateLayeredWindow.Call(
		o.hwnd, screenDC, uintptr(unsafe.Pointer(&ptDst)), uintptr(unsafe.Pointer(&sz)),
		memDC, uintptr(unsafe.Pointer(&ptSrc)), 0,
		uintptr(unsafe.Pointer(&blend)), ULW_ALPHA,
	)

	pDeleteObject.Call(hBmp)
	pDeleteDC.Call(memDC)
	pReleaseDC.Call(0, screenDC)
}

// ── Drawing primitives ──

func drawCircle(px *[1 << 25]byte, w, h, cx, cy, radius int, r, g, b byte, alpha byte) {
	fr := float64(radius)
	for y := max(0, cy-radius-2); y < min(h, cy+radius+2); y++ {
		for x := max(0, cx-radius-2); x < min(w, cx+radius+2); x++ {
			dx := float64(x) - float64(cx)
			dy := float64(y) - float64(cy)
			dist := math.Sqrt(dx*dx+dy*dy) - fr
			if dist < -1.0 {
				blendPixel(px, w, x, y, r, g, b, alpha)
			} else if dist < 1.0 {
				a := byte(float64(alpha) * (1.0 - (dist+1.0)/2.0))
				blendPixel(px, w, x, y, r, g, b, a)
			}
		}
	}
}

func drawRoundedRect(px *[1 << 25]byte, w, h, rx, ry, rw, rh, radius int, cr, cg, cb byte, alpha byte) {
	rad := float64(radius)
	for y := ry; y < ry+rh && y < h; y++ {
		for x := rx; x < rx+rw && x < w; x++ {
			lx := float64(x - rx)
			ly := float64(y - ry)
			fw := float64(rw)
			fh := float64(rh)
			dist := roundedRectDist(lx, ly, fw, fh, rad)
			if dist < -1.0 {
				blendPixel(px, w, x, y, cr, cg, cb, alpha)
			} else if dist < 1.0 {
				a := byte(float64(alpha) * (1.0 - (dist+1.0)/2.0))
				blendPixel(px, w, x, y, cr, cg, cb, a)
			}
		}
	}
}

func drawRoundedRectBorder(px *[1 << 25]byte, w, h, rx, ry, rw, rh, radius int, cr, cg, cb byte, alpha byte) {
	rad := float64(radius)
	for y := ry; y < ry+rh && y < h; y++ {
		for x := rx; x < rx+rw && x < w; x++ {
			lx := float64(x - rx)
			ly := float64(y - ry)
			fw := float64(rw)
			fh := float64(rh)
			dist := roundedRectDist(lx, ly, fw, fh, rad)
			if dist > -1.5 && dist < 0.5 {
				t := 1.0 - math.Abs(dist+0.5)
				if t > 0 {
					a := byte(float64(alpha) * t)
					blendPixel(px, w, x, y, cr, cg, cb, a)
				}
			}
		}
	}
}

func roundedRectDist(px, py, w, h, r float64) float64 {
	cx := px - w/2
	cy := py - h/2
	dx := math.Abs(cx) - (w/2 - r)
	dy := math.Abs(cy) - (h/2 - r)
	outside := math.Sqrt(math.Max(dx, 0)*math.Max(dx, 0)+math.Max(dy, 0)*math.Max(dy, 0)) - r
	inside := math.Min(math.Max(dx, dy), 0)
	return outside + inside
}

func drawCheck(px *[1 << 25]byte, stride, x, y int, r, g, b, a byte) {
	// Simple checkmark: ✓
	pts := [][2]int{
		{x, y + 1}, {x + 1, y + 2}, {x + 2, y + 3},
		{x + 3, y + 2}, {x + 4, y + 1}, {x + 5, y},
	}
	for _, p := range pts {
		setPixel(px, stride, p[0], p[1], r, g, b, a)
		if p[1]+1 < 40 {
			setPixel(px, stride, p[0], p[1]+1, r, g, b, byte(float64(a)*0.5))
		}
	}
}

func drawX(px *[1 << 25]byte, stride, cx, cy int, r, g, b, a byte) {
	for i := -2; i <= 2; i++ {
		setPixel(px, stride, cx+i, cy+i, r, g, b, a)
		setPixel(px, stride, cx+i, cy-i, r, g, b, a)
	}
}

func setPixel(px *[1 << 25]byte, stride, x, y int, r, g, b, a byte) {
	if x < 0 || y < 0 || x >= stride || y >= 40 {
		return
	}
	off := (y*stride + x) * 4
	fa := float64(a) / 255.0
	px[off+0] = byte(float64(b) * fa)
	px[off+1] = byte(float64(g) * fa)
	px[off+2] = byte(float64(r) * fa)
	px[off+3] = a
}

func blendPixel(px *[1 << 25]byte, stride, x, y int, r, g, b, a byte) {
	if x < 0 || y < 0 || x >= stride || y >= 40 {
		return
	}
	off := (y*stride + x) * 4
	if px[off+3] == 0 {
		setPixel(px, stride, x, y, r, g, b, a)
		return
	}
	fa := float64(a) / 255.0
	invA := 1.0 - fa
	px[off+0] = byte(float64(b)*fa + float64(px[off+0])*invA)
	px[off+1] = byte(float64(g)*fa + float64(px[off+1])*invA)
	px[off+2] = byte(float64(r)*fa + float64(px[off+2])*invA)
	newA := float64(a) + float64(px[off+3])*invA
	if newA > 255 {
		newA = 255
	}
	px[off+3] = byte(newA)
}

// drawTextGDI renders text into pixel buffer using GDI with proper alpha compositing.
func drawTextGDI(px *[1 << 25]byte, w, h int, text string, _ uintptr, left, top, right, bottom int, tr, tg, tb byte) {
	screenDC, _, _ := pGetDC.Call(0)
	memDC2, _, _ := pCreateCompatibleDC.Call(screenDC)

	bmi := BITMAPINFOHEADER{
		Size:     uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		Width:    int32(w),
		Height:   -int32(h),
		Planes:   1,
		BitCount: 32,
	}
	var textBits uintptr
	textBmp, _, _ := pCreateDIBSection.Call(memDC2, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&textBits)), 0, 0)
	pSelectObject.Call(memDC2, textBmp)

	textPx := (*[1 << 25]byte)(unsafe.Pointer(textBits))
	for i := 0; i < w*h*4; i++ {
		textPx[i] = 0
	}

	pSetBkMode.Call(memDC2, TRANSPARENT)
	pSetTextColor.Call(memDC2, 0x00FFFFFF)

	fontName, _ := syscall.UTF16PtrFromString("Segoe UI Semibold")
	font, _, _ := pCreateFont.Call(
		uintptr(^uint32(14)+1), 0, 0, 0, 600, 0, 0, 0, 0, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	pSelectObject.Call(memDC2, font)

	textStr, _ := syscall.UTF16PtrFromString(text)
	textRect := RECT{int32(left), int32(top), int32(right), int32(bottom)}
	pDrawText.Call(memDC2, uintptr(unsafe.Pointer(textStr)), ^uintptr(0),
		uintptr(unsafe.Pointer(&textRect)), DT_SINGLELINE|DT_VCENTER)

	// Composite: use brightness as alpha
	for y := 0; y < h; y++ {
		for x := left; x < right && x < w; x++ {
			off := (y*w + x) * 4
			r := textPx[off+2]
			g := textPx[off+1]
			b := textPx[off+0]
			mx := r
			if g > mx {
				mx = g
			}
			if b > mx {
				mx = b
			}
			if mx > 10 {
				blendPixel(px, w, x, y, tr, tg, tb, mx)
			}
		}
	}

	pDeleteObject.Call(font)
	pDeleteObject.Call(textBmp)
	pDeleteDC.Call(memDC2)
	pReleaseDC.Call(0, screenDC)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
