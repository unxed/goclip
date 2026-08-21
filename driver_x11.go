package goclip

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// X11Driver communicates directly with the X11 display server using pure-Go XGB.
type X11Driver struct {
	mu          sync.Mutex
	conn        *xgb.Conn
	win         xproto.Window
	clipboard   xproto.Atom
	utf8String  xproto.Atom
	targets     xproto.Atom
	goclipProp  xproto.Atom
	currentData string
	serving     bool
	serveStop   chan struct{}
}

func NewX11Driver() *X11Driver {
	return &X11Driver{}
}

func (d *X11Driver) Name() string {
	return "x11"
}

func (d *X11Driver) Available() bool {
	if os.Getenv("DISPLAY") == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		return true
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return false
	}
	_ = d.initAtoms(conn)
	d.conn = conn
	return true
}

func (d *X11Driver) initAtoms(conn *xgb.Conn) error {
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)
	win, err := xproto.NewWindowId(conn)
	if err != nil {
		return err
	}

	err = xproto.CreateWindowChecked(
		conn,
		screen.RootDepth,
		win,
		screen.Root,
		0, 0, 1, 1, 0,
		xproto.WindowClassInputOnly,
		screen.RootVisual,
		0, []uint32{},
	).Check()
	if err != nil {
		return err
	}

	internAtom := func(name string) xproto.Atom {
		r, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
		if err != nil {
			return 0
		}
		return r.Atom
	}

	d.win = win
	d.clipboard = internAtom("CLIPBOARD")
	d.utf8String = internAtom("UTF8_STRING")
	d.targets = internAtom("TARGETS")
	d.goclipProp = internAtom("GOCLIP_SELECTION")
	return nil
}

func (d *X11Driver) ReadText() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn == nil {
		conn, err := xgb.NewConn()
		if err != nil {
			return "", err
		}
		if err := d.initAtoms(conn); err != nil {
			conn.Close()
			return "", err
		}
		d.conn = conn
	}

	// Request ownership transfer
	_ = xproto.ConvertSelection(
		d.conn,
		d.win,
		d.clipboard,
		d.utf8String,
		d.goclipProp,
		xproto.TimeCurrentTime,
	)

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		ev, err := d.conn.PollForEvent()
		if err != nil {
			return "", err
		}
		if ev == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if sn, ok := ev.(xproto.SelectionNotifyEvent); ok {
			if sn.Property == xproto.AtomNone {
				return "", ErrEmpty
			}
			propReply, err := xproto.GetProperty(
				d.conn,
				true,
				d.win,
				sn.Property,
				xproto.GetPropertyTypeAny,
				0, 1024*1024,
			).Reply()
			if err != nil {
				return "", err
			}
			return string(propReply.Value), nil
		}
	}
	return "", errors.New("x11 selection request timed out")
}

func (d *X11Driver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.conn == nil {
		conn, err := xgb.NewConn()
		if err != nil {
			return err
		}
		if err := d.initAtoms(conn); err != nil {
			conn.Close()
			return err
		}
		d.conn = conn
	}

	d.currentData = text
	err := xproto.SetSelectionOwnerChecked(d.conn, d.win, d.clipboard, xproto.TimeCurrentTime).Check()
	if err != nil {
		return err
	}

	if !d.serving {
		d.serving = true
		d.serveStop = make(chan struct{})
		go d.eventLoop()
	}
	return nil
}

func (d *X11Driver) eventLoop() {
	for {
		d.mu.Lock()
		conn := d.conn
		d.mu.Unlock()
		if conn == nil {
			return
		}

		ev, err := conn.WaitForEvent()
		if err != nil || ev == nil {
			return
		}

		switch event := ev.(type) {
		case xproto.SelectionRequestEvent:
			d.mu.Lock()
			if event.Selection == d.clipboard {
				if event.Target == d.targets {
					targets := []xproto.Atom{d.targets, d.utf8String, xproto.AtomString}
					raw := make([]byte, len(targets)*4)
					for i, target := range targets {
						xgb.Put32(raw[i*4:], uint32(target))
					}
					_ = xproto.ChangePropertyChecked(
						d.conn,
						xproto.PropModeReplace,
						event.Requestor,
						event.Property,
						xproto.AtomAtom,
						32,
						uint32(len(targets)),
						raw,
					).Check()
				} else if event.Target == d.utf8String || event.Target == xproto.AtomString {
					_ = xproto.ChangePropertyChecked(
						d.conn,
						xproto.PropModeReplace,
						event.Requestor,
						event.Property,
						event.Target,
						8,
						uint32(len(d.currentData)),
						[]byte(d.currentData),
					).Check()
				}
				xproto.SendEvent(
					d.conn,
					false,
					event.Requestor,
					0,
					string(xproto.SelectionNotifyEvent{
						Time:      event.Time,
						Requestor: event.Requestor,
						Selection: event.Selection,
						Target:    event.Target,
						Property:  event.Property,
					}.Bytes()),
				)
			}
			d.mu.Unlock()
		case xproto.SelectionClearEvent:
			d.mu.Lock()
			if event.Selection == d.clipboard {
				d.serving = false
				d.mu.Unlock()
				return
			}
			d.mu.Unlock()
		}
	}
}

func (d *X11Driver) Clear() error {
	return d.WriteText("")
}
