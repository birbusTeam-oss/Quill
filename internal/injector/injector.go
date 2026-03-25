package injector

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Windows clipboard + SendInput via syscall — zero third-party deps.

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	getClipboardData = user32.NewProc("GetClipboardData")
	sendInput        = user32.NewProc("SendInput")
	getKeyState      = user32.NewProc("GetKeyState")

	globalAlloc  = kernel32.NewProc("GlobalAlloc")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")
	globalSize   = kernel32.NewProc("GlobalSize")
	lstrcpy      = kernel32.NewProc("lstrcpyW")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	inputKeyboard = 1
	keyEventUp    = 0x0002

	vkControl = 0x11
	vkV       = 0x56
)

// INPUT structure for SendInput
type keyboardInput struct {
	Type uint32
	Ki   keyInput
	_    [8]byte // padding to match union size
}

type keyInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// InjectText copies text to clipboard and simulates Ctrl+V.
func InjectText(text string) error {
	if text == "" {
		return nil
	}

	// Save old clipboard content
	oldClip := getClipboardText()

	// Set new clipboard text
	if err := setClipboardText(text); err != nil {
		return fmt.Errorf("set clipboard: %w", err)
	}

	// Small delay to let clipboard settle
	time.Sleep(50 * time.Millisecond)

	// Simulate Ctrl+V
	if err := simulateCtrlV(); err != nil {
		return fmt.Errorf("simulate paste: %w", err)
	}

	// Wait for paste to complete
	time.Sleep(100 * time.Millisecond)

	// Restore old clipboard
	if oldClip != "" {
		_ = setClipboardText(oldClip)
	}

	return nil
}

func getClipboardText() string {
	ret, _, _ := openClipboard.Call(0)
	if ret == 0 {
		return ""
	}
	defer closeClipboard.Call()

	h, _, _ := getClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer globalUnlock.Call(h)

	sz, _, _ := globalSize.Call(h)
	if sz == 0 {
		return ""
	}

	// Read UTF-16 string
	n := int(sz / 2)
	if n <= 0 {
		return ""
	}
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), n)
	var result []uint16
	for _, ch := range buf {
		if ch == 0 {
			break
		}
		result = append(result, ch)
	}
	return syscall.UTF16ToString(result)
}

func setClipboardText(text string) error {
	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	ret, _, _ := openClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("OpenClipboard failed")
	}
	defer closeClipboard.Call()

	emptyClipboard.Call()

	size := len(utf16) * 2
	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := globalLock.Call(h)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed")
	}

	// Copy UTF-16 data
	dest := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16))
	copy(dest, utf16)
	globalUnlock.Call(h)

	ret, _, _ = setClipboardData.Call(cfUnicodeText, h)
	if ret == 0 {
		return fmt.Errorf("SetClipboardData failed")
	}
	return nil
}

func simulateCtrlV() error {
	inputs := []keyboardInput{
		// Ctrl down
		{Type: inputKeyboard, Ki: keyInput{Vk: vkControl}},
		// V down
		{Type: inputKeyboard, Ki: keyInput{Vk: vkV}},
		// V up
		{Type: inputKeyboard, Ki: keyInput{Vk: vkV, Flags: keyEventUp}},
		// Ctrl up
		{Type: inputKeyboard, Ki: keyInput{Vk: vkControl, Flags: keyEventUp}},
	}

	ret, _, _ := sendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret == 0 {
		return fmt.Errorf("SendInput failed")
	}
	return nil
}
