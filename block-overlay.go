package main

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

type BlockOverlay struct {
	X        *xgb.Conn
	screen   *xproto.ScreenInfo
	root     xproto.Window
	overlay  xproto.Window
	isActive bool
	mu       sync.Mutex
}

func NewBlockOverlay(X *xgb.Conn) (*BlockOverlay, error) {
	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)
	root := screen.Root

	bo := &BlockOverlay{
		X:      X,
		screen: screen,
		root:   root,
	}

	// Create the overlay window once at startup
	err := bo.createOverlayWindow()
	if err != nil {
		return nil, fmt.Errorf("failed to create overlay window: %v", err)
	}

	return bo, nil
}

func (bo *BlockOverlay) createOverlayWindow() error {
	width := bo.screen.WidthInPixels
	height := bo.screen.HeightInPixels

	wid, err := xproto.NewWindowId(bo.X)
	if err != nil {
		return err
	}

	const CopyFromParent = 0
	mask := uint32(xproto.CwBackPixel | xproto.CwOverrideRedirect | xproto.CwEventMask)
	values := []uint32{
		0x00000000, // CwBackPixel - black (will be made transparent)
		1,          // CwOverrideRedirect - bypass window manager
		uint32(xproto.EventMaskKeyPress |
			xproto.EventMaskKeyRelease |
			xproto.EventMaskButtonPress |
			xproto.EventMaskButtonRelease |
			xproto.EventMaskPointerMotion |
			xproto.EventMaskStructureNotify |
			xproto.EventMaskVisibilityChange),
	}

	err = xproto.CreateWindowChecked(bo.X,
		CopyFromParent,
		wid,
		bo.root,
		0, 0,
		width, height,
		0,
		xproto.WindowClassInputOutput,
		CopyFromParent,
		mask,
		values).Check()

	if err != nil {
		return err
	}

	bo.overlay = wid

	// Set window properties for transparency and fullscreen
	bo.setWindowProperties()

	log.Printf("Overlay window created (ID: %d, Size: %dx%d)\n", wid, width, height)
	return nil
}

// Show makes the overlay visible and grabs input
func (bo *BlockOverlay) Show() error {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	if bo.isActive {
		return nil // Already visible
	}

	// Map the window (make it visible)
	err := xproto.MapWindowChecked(bo.X, bo.overlay).Check()
	if err != nil {
		return fmt.Errorf("failed to map window: %v", err)
	}

	// Raise window to top of stack
	err = xproto.ConfigureWindowChecked(bo.X, bo.overlay,
		xproto.ConfigWindowStackMode,
		[]uint32{xproto.StackModeAbove}).Check()
	if err != nil {
		return fmt.Errorf("failed to raise window: %v", err)
	}

	// Set input focus to our window
	xproto.SetInputFocus(bo.X, xproto.InputFocusPointerRoot,
		bo.overlay, xproto.TimeCurrentTime)

	// Sync to ensure commands are processed
	bo.X.Sync()

	// Small delay to ensure window is fully mapped before grabbing input
	time.Sleep(50 * time.Millisecond)

	// Grab all input
	bo.grabInput()

	bo.isActive = true
	log.Println("Overlay shown - blocking all input")
	return nil
}

// Hide makes the overlay invisible and releases input
func (bo *BlockOverlay) Hide() error {
	bo.mu.Lock()
	defer bo.mu.Unlock()

	if !bo.isActive {
		return nil // Already hidden
	}

	// Release input grabs first
	xproto.UngrabKeyboard(bo.X, xproto.TimeCurrentTime)
	xproto.UngrabPointer(bo.X, xproto.TimeCurrentTime)

	// Unmap the window (hide it but keep it alive)
	err := xproto.UnmapWindowChecked(bo.X, bo.overlay).Check()
	if err != nil {
		// If unmapping fails, try lowering it instead
		err2 := xproto.ConfigureWindowChecked(bo.X, bo.overlay,
			xproto.ConfigWindowStackMode,
			[]uint32{xproto.StackModeBelow}).Check()

		if err2 != nil {
			return errors.Join(errors.New("failed to hide window"), err, err2)
		}
	}

	bo.X.Sync()

	bo.isActive = false
	log.Println("Overlay hidden - input restored")
	return nil
}

// IsActive returns whether the overlay is currently visible
func (bo *BlockOverlay) IsActive() bool {
	bo.mu.Lock()
	defer bo.mu.Unlock()
	return bo.isActive
}

// Destroy cleans up the overlay window
func (bo *BlockOverlay) Destroy() error {
	bo.Hide() // Ensure we release grabs

	if bo.overlay != 0 {
		err := xproto.DestroyWindowChecked(bo.X, bo.overlay).Check()
		bo.overlay = 0
		return err
	}
	return nil
}

// grabInput grabs keyboard and pointer input
func (bo *BlockOverlay) grabInput() {
	// Grab pointer (mouse/touch)
	pointerReply, err := xproto.GrabPointer(bo.X,
		false,      // owner_events
		bo.overlay, // grab_window
		uint16(xproto.EventMaskButtonPress|
			xproto.EventMaskButtonRelease|
			xproto.EventMaskPointerMotion), // event_mask
		xproto.GrabModeAsync,           // pointer_mode
		xproto.GrabModeAsync,           // keyboard_mode
		xproto.WindowNone,              // confine_to
		xproto.CursorNone,              // cursor
		xproto.TimeCurrentTime).Reply() // time

	if err != nil {
		log.Printf("Warning: failed to grab pointer: %v", err)
	} else if pointerReply.Status != xproto.GrabStatusSuccess {
		log.Printf("Warning: bad pointer grab status: %d", pointerReply.Status)
	}
}

func (bo *BlockOverlay) setWindowProperties() {
	// Set window type to dock for staying on top
	netWmWindowType := internAtom(bo.X, "_NET_WM_WINDOW_TYPE")
	netWmWindowTypeDock := internAtom(bo.X, "_NET_WM_WINDOW_TYPE_DOCK")

	if netWmWindowType != 0 && netWmWindowTypeDock != 0 {
		data := make([]byte, 4)
		put32(data, uint32(netWmWindowTypeDock))
		xproto.ChangePropertyChecked(bo.X,
			xproto.PropModeReplace,
			bo.overlay,
			netWmWindowType,
			xproto.AtomAtom,
			32,
			1,
			data).Check()
	}

	// Set window states for fullscreen and above
	netWmState := internAtom(bo.X, "_NET_WM_STATE")
	var states []xproto.Atom

	// desired states
	stateNames := []string{
		"_NET_WM_STATE_FULLSCREEN",
		"_NET_WM_STATE_ABOVE",
		"_NET_WM_STATE_SKIP_TASKBAR",
		"_NET_WM_STATE_SKIP_PAGER",
	}

	for _, name := range stateNames {
		if atom := internAtom(bo.X, name); atom != 0 {
			states = append(states, atom)
		}
	}

	if netWmState != 0 && len(states) > 0 {
		data := make([]byte, len(states)*4)
		for i, state := range states {
			put32(data[i*4:], uint32(state))
		}
		xproto.ChangePropertyChecked(bo.X,
			xproto.PropModeReplace,
			bo.overlay,
			netWmState,
			xproto.AtomAtom,
			32,
			uint32(len(states)),
			data).Check()
	}

	// Set transparency (requires compositor)
	netWmWindowOpacity := internAtom(bo.X, "_NET_WM_WINDOW_OPACITY")
	if netWmWindowOpacity != 0 {
		opacity := uint32(0x00000000) // Fully transparent
		data := make([]byte, 4)
		put32(data, opacity)
		xproto.ChangePropertyChecked(bo.X,
			xproto.PropModeReplace,
			bo.overlay,
			netWmWindowOpacity,
			xproto.AtomCardinal,
			32,
			1,
			data).Check()
	}
}

func internAtom(X *xgb.Conn, name string) xproto.Atom {
	reply, err := xproto.InternAtom(X, false, uint16(len(name)), name).Reply()
	if err != nil {
		log.Printf("Failed to intern atom %s: %v", name, err)
		return 0
	}
	return reply.Atom
}

func put32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
