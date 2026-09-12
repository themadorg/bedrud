//go:build linux

package tray

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	traySize = 16
	dockOp   = 0 // SYSTEM_TRAY_REQUEST_DOCK
)

func startXEmbed(cb Callbacks) (func(), error) {
	X, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)
	sid := screen.Root

	name := fmt.Sprintf("_NET_SYSTEM_TRAY_S%d", X.DefaultScreen)
	sel, err := intern(X, name)
	if err != nil {
		X.Close()
		return nil, err
	}
	own, err := xproto.GetSelectionOwner(X, sel).Reply()
	if err != nil {
		X.Close()
		return nil, err
	}
	if own.Owner == 0 {
		X.Close()
		return nil, fmt.Errorf("no XEmbed tray (enable polybar internal/tray)")
	}

	wid, err := xproto.NewWindowId(X)
	if err != nil {
		X.Close()
		return nil, err
	}
	// Override-redirect so i3/X11 never tiles this as a normal client.
	// Do not MapWindow: the tray embedder maps after REQUEST_DOCK.
	mask := uint32(xproto.CwBackPixel | xproto.CwBorderPixel | xproto.CwEventMask | xproto.CwOverrideRedirect)
	vals := []uint32{
		0x00e11d48,
		0x00e11d48,
		xproto.EventMaskButtonPress | xproto.EventMaskExposure | xproto.EventMaskStructureNotify,
		1,
	}
	xproto.CreateWindow(X, screen.RootDepth, wid, sid, 0, 0, traySize, traySize, 0,
		xproto.WindowClassInputOutput, screen.RootVisual, mask, vals)

	if infoAtom, err := intern(X, "_XEMBED_INFO"); err == nil {
		data := make([]byte, 8)
		data[4] = 1 // XEMBED_MAPPED — ask embedder to map
		xproto.ChangeProperty(X, xproto.PropModeReplace, wid, infoAtom, infoAtom, 32, 2, data)
	}
	if skip, err := intern(X, "_NET_WM_STATE_SKIP_TASKBAR"); err == nil {
		if state, err := intern(X, "_NET_WM_STATE"); err == nil {
			buf := make([]byte, 8)
			xgb.Put32(buf[0:], uint32(skip))
			if pager, err := intern(X, "_NET_WM_STATE_SKIP_PAGER"); err == nil {
				xgb.Put32(buf[4:], uint32(pager))
			}
			xproto.ChangeProperty(X, xproto.PropModeReplace, wid, state, xproto.AtomAtom, 32, 2, buf)
		}
	}

	op, err := intern(X, "_NET_SYSTEM_TRAY_OPCODE")
	if err != nil {
		X.Close()
		return nil, err
	}
	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: own.Owner,
		Type:   op,
		Data:   cm32(0, dockOp, uint32(wid), 0, 0),
	}
	xproto.SendEvent(X, false, own.Owner, xproto.EventMaskNoEvent, string(ev.Bytes()))

	stop := make(chan struct{})
	cleanup := func() {
		_ = xproto.UnmapWindow(X, wid)
		_ = xproto.DestroyWindow(X, wid)
		X.Close()
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		select {
		case <-stop:
		case <-sig:
		}
		signal.Stop(sig)
		cleanup()
	}()
	go func() {
		for {
			raw, err := X.WaitForEvent()
			if err != nil || raw == nil {
				return
			}
			switch raw.(type) {
			case xproto.ButtonPressEvent:
				if cb.Toggle != nil {
					cb.Toggle()
				}
			case xproto.ExposeEvent, xproto.MapNotifyEvent, xproto.ReparentNotifyEvent:
				paintIcon(X, wid)
			}
		}
	}()
	log.Printf("xembed tray requested dock on %s (unmapped, override-redirect)", name)
	return func() { close(stop) }, nil
}

func intern(X *xgb.Conn, name string) (xproto.Atom, error) {
	r, err := xproto.InternAtom(X, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return r.Atom, nil
}

func cm32(a, b, c, d, e uint32) xproto.ClientMessageDataUnion {
	return xproto.ClientMessageDataUnionData32New([]uint32{a, b, c, d, e})
}

func paintIcon(X *xgb.Conn, wid xproto.Window) {
	gc, err := xproto.NewGcontextId(X)
	if err != nil {
		return
	}
	xproto.CreateGC(X, gc, xproto.Drawable(wid), xproto.GcForeground, []uint32{0x00e11d48})
	xproto.PolyFillRectangle(X, xproto.Drawable(wid), gc, []xproto.Rectangle{{X: 0, Y: 0, Width: traySize, Height: traySize}})
	xproto.ChangeGC(X, gc, xproto.GcForeground, []uint32{0x00ffffff})
	xproto.PolyFillRectangle(X, xproto.Drawable(wid), gc, []xproto.Rectangle{
		{X: 3, Y: 4, Width: 10, Height: 2},
		{X: 3, Y: 7, Width: 10, Height: 2},
		{X: 3, Y: 10, Width: 10, Height: 2},
	})
	xproto.FreeGC(X, gc)
}
