package goclip

// The X11 clipboard, spoken directly to the server through XGB.
//
// A selection is not a place where text is kept. It is a claim: an application
// says it owns CLIPBOARD, and every paste afterwards is a conversation between
// the pasting application and the owning one. That is why this driver needs a
// window, an event loop, and a way to answer requests for as long as it holds
// the selection — and why a value larger than one X request has to be handed
// over in pieces, in both directions.

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	// x11ReadTimeout bounds a paste. The owner of the selection is another
	// application; it may be wedged, or gone in a way the server has not
	// noticed yet.
	x11ReadTimeout = 2 * time.Second

	// x11MaxTransfer bounds what will be accepted, so that an application
	// offering an endless incremental transfer cannot exhaust memory.
	x11MaxTransfer = 64 << 20

	// x11ChunkSize is how much of a large value goes in one piece. A request
	// carries its length in four byte units in a sixteen bit field, so a
	// quarter of a megabyte is the ceiling; this leaves room under it.
	x11ChunkSize = 128 << 10
)

// X11Driver communicates directly with the X11 display server using pure-Go XGB.
type X11Driver struct {
	mu   sync.Mutex
	conn *xgb.Conn
	win  xproto.Window

	clipboard  xproto.Atom
	utf8String xproto.Atom
	targets    xproto.Atom
	timestamp  xproto.Atom
	textAtom   xproto.Atom
	incr       xproto.Atom
	goclipProp xproto.Atom

	currentData string
	owning      bool

	// The paste in flight, if any.
	reading  bool
	readDone chan string
	readBuf  []byte
	readIncr bool

	// The copies in flight: one per requestor that asked for a value too
	// large to hand over in a single request.
	sending map[xproto.Window]*x11OutgoingTransfer
}

type x11OutgoingTransfer struct {
	prop xproto.Atom
	rest string
}

func NewX11Driver() *X11Driver {
	return &X11Driver{sending: make(map[xproto.Window]*x11OutgoingTransfer)}
}

func (d *X11Driver) Name() string { return "x11" }

func (d *X11Driver) Available() bool {
	if os.Getenv("DISPLAY") == "" {
		return false
	}
	return d.ensure() == nil
}

// ensure connects, creates the window and starts the event loop, once.
func (d *X11Driver) ensure() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		return nil
	}
	if os.Getenv("DISPLAY") == "" {
		return errors.New("goclip: there is no X display")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return err
	}
	if err := d.initAtoms(conn); err != nil {
		conn.Close()
		return err
	}
	d.conn = conn
	go d.eventLoop(conn)
	return nil
}

func (d *X11Driver) initAtoms(conn *xgb.Conn) error {
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)
	win, err := xproto.NewWindowId(conn)
	if err != nil {
		return err
	}

	// InputOutput and not InputOnly. An InputOnly window must be created
	// with depth zero and the CopyFromParent visual; creating one with the
	// root depth and visual is a BadMatch, which is what this driver used to
	// do — so the native path failed on every server, on every call, and
	// every copy and paste quietly fell through to xclip, which is exactly
	// what this driver exists not to need.
	//
	// PropertyChange is selected because an incremental transfer arrives as
	// a series of changes to a property of this window.
	err = xproto.CreateWindowChecked(
		conn,
		screen.RootDepth,
		win,
		screen.Root,
		0, 0, 1, 1, 0,
		xproto.WindowClassInputOutput,
		screen.RootVisual,
		xproto.CwEventMask,
		[]uint32{uint32(xproto.EventMaskPropertyChange)},
	).Check()
	if err != nil {
		return err
	}

	internAtom := func(name string) xproto.Atom {
		r, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
		if err != nil || r == nil {
			return 0
		}
		return r.Atom
	}

	d.win = win
	d.clipboard = internAtom("CLIPBOARD")
	d.utf8String = internAtom("UTF8_STRING")
	d.targets = internAtom("TARGETS")
	d.timestamp = internAtom("TIMESTAMP")
	d.textAtom = internAtom("TEXT")
	d.incr = internAtom("INCR")
	d.goclipProp = internAtom("GOCLIP_SELECTION")
	if d.clipboard == 0 || d.utf8String == 0 || d.goclipProp == 0 {
		return errors.New("goclip: the selection atoms could not be interned")
	}
	return nil
}

// ReadText pastes: it asks whoever owns CLIPBOARD for the value and waits for
// the answer, which arrives through the event loop.
func (d *X11Driver) ReadText() (string, error) {
	if err := d.ensure(); err != nil {
		return "", err
	}

	d.mu.Lock()
	// A selection we own ourselves is answered from memory: asking the
	// server would mean answering our own request from the goroutine that
	// would have to deliver it.
	if d.owning {
		text := d.currentData
		d.mu.Unlock()
		if text == "" {
			return "", ErrEmpty
		}
		return text, nil
	}
	if d.reading {
		d.mu.Unlock()
		return "", errors.New("goclip: another paste is already in flight")
	}
	done := make(chan string, 1)
	d.reading, d.readDone, d.readBuf, d.readIncr = true, done, nil, false
	xproto.ConvertSelection(d.conn, d.win, d.clipboard, d.utf8String,
		d.goclipProp, xproto.TimeCurrentTime)
	d.mu.Unlock()

	select {
	case text := <-done:
		if text == "" {
			return "", ErrEmpty
		}
		return text, nil
	case <-time.After(x11ReadTimeout):
		d.mu.Lock()
		d.reading, d.readDone = false, nil
		d.mu.Unlock()
		return "", errors.New("goclip: the owner of the selection did not answer")
	}
}

// WriteText copies: it takes the selection and then answers, for as long as it
// holds it, every application that asks what is on the clipboard.
func (d *X11Driver) WriteText(text string) error {
	if err := d.ensure(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.currentData = text
	if err := xproto.SetSelectionOwnerChecked(d.conn, d.win, d.clipboard,
		xproto.TimeCurrentTime).Check(); err != nil {
		return err
	}
	d.owning = true
	return nil
}

func (d *X11Driver) Clear() error { return d.WriteText("") }

// eventLoop is the only reader of the connection. Polling for events from
// ReadText while this also ran was the other half of why the native path did
// not work: two consumers of one event stream, each able to swallow what the
// other was waiting for.
func (d *X11Driver) eventLoop(conn *xgb.Conn) {
	for {
		ev, err := conn.WaitForEvent()
		if ev == nil && err == nil {
			return
		}
		if ev == nil {
			continue
		}
		switch e := ev.(type) {
		case xproto.SelectionNotifyEvent:
			d.onSelectionNotify(e)
		case xproto.PropertyNotifyEvent:
			d.onPropertyNotify(e)
		case xproto.SelectionRequestEvent:
			d.onSelectionRequest(e)
		case xproto.SelectionClearEvent:
			d.mu.Lock()
			if e.Selection == d.clipboard {
				d.owning = false
			}
			d.mu.Unlock()
		}
	}
}

func (d *X11Driver) onSelectionNotify(e xproto.SelectionNotifyEvent) {
	d.mu.Lock()
	reading := d.reading
	incr := d.incr
	d.mu.Unlock()
	if !reading {
		return
	}

	if e.Property == xproto.AtomNone {
		d.finishRead("")
		return
	}
	data, typ, ok := d.takeProperty(e.Property)
	if !ok {
		d.finishRead("")
		return
	}
	if incr != 0 && typ == incr {
		// The value follows in pieces. Deleting the property, which
		// takeProperty has already done, is what tells the owner to start
		// sending them.
		d.mu.Lock()
		d.readIncr, d.readBuf = true, nil
		d.mu.Unlock()
		return
	}
	d.finishRead(string(data))
}

func (d *X11Driver) onPropertyNotify(e xproto.PropertyNotifyEvent) {
	d.mu.Lock()
	mine := e.Window == d.win
	incoming := d.reading && d.readIncr
	d.mu.Unlock()

	if mine && incoming && e.State == xproto.PropertyNewValue {
		data, _, ok := d.takeProperty(e.Atom)
		if !ok {
			d.finishRead("")
			return
		}
		if len(data) == 0 {
			d.mu.Lock()
			text := string(d.readBuf)
			d.mu.Unlock()
			d.finishRead(text)
			return
		}
		d.mu.Lock()
		if len(d.readBuf)+len(data) > x11MaxTransfer {
			d.mu.Unlock()
			d.finishRead("")
			return
		}
		d.readBuf = append(d.readBuf, data...)
		d.mu.Unlock()
		return
	}

	// A requestor deleting the property is it asking for the next piece of
	// something we are handing over.
	if !mine && e.State == xproto.PropertyDelete {
		d.sendNextChunk(e.Window)
	}
}

// takeProperty reads a property of our own window whole and deletes it, which
// in a selection transfer is also the acknowledgement.
func (d *X11Driver) takeProperty(prop xproto.Atom) ([]byte, xproto.Atom, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn == nil {
		return nil, 0, false
	}

	var out []byte
	var typ xproto.Atom
	for off := uint32(0); ; {
		// A reply is capped by the server, which says what is left in
		// BytesAfter. Reading one reply and stopping, which is what this
		// used to do, silently truncated anything large.
		reply, err := xproto.GetProperty(d.conn, false, d.win, prop,
			xproto.GetPropertyTypeAny, off, 16384).Reply()
		if err != nil || reply == nil {
			return nil, 0, false
		}
		typ = reply.Type
		out = append(out, reply.Value...)
		if reply.BytesAfter == 0 || len(reply.Value) == 0 {
			break
		}
		if len(out) > x11MaxTransfer {
			return nil, 0, false
		}
		off += uint32(len(reply.Value)) / 4
	}
	xproto.DeleteProperty(d.conn, d.win, prop)
	return out, typ, true
}

func (d *X11Driver) finishRead(text string) {
	d.mu.Lock()
	done := d.readDone
	d.reading, d.readDone, d.readBuf, d.readIncr = false, nil, nil, false
	d.mu.Unlock()
	if done != nil {
		done <- text
	}
}

// onSelectionRequest answers another application's paste.
func (d *X11Driver) onSelectionRequest(e xproto.SelectionRequestEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn == nil || e.Selection != d.clipboard || !d.owning {
		d.notify(e, xproto.AtomNone)
		return
	}

	prop := e.Property
	if prop == xproto.AtomNone {
		// An obsolete requestor; the convention is to use the target.
		prop = e.Target
	}

	switch e.Target {
	case d.targets:
		list := []xproto.Atom{d.targets, d.timestamp, d.utf8String, xproto.AtomString, d.textAtom}
		raw := make([]byte, 0, len(list)*4)
		for _, a := range list {
			var b [4]byte
			xgb.Put32(b[:], uint32(a))
			raw = append(raw, b[:]...)
		}
		xproto.ChangeProperty(d.conn, xproto.PropModeReplace, e.Requestor, prop,
			xproto.AtomAtom, 32, uint32(len(list)), raw)

	case d.timestamp:
		var b [4]byte
		xgb.Put32(b[:], uint32(xproto.TimeCurrentTime))
		xproto.ChangeProperty(d.conn, xproto.PropModeReplace, e.Requestor, prop,
			xproto.AtomInteger, 32, 1, b[:])

	case d.utf8String, xproto.AtomString, d.textAtom:
		typ := e.Target
		if typ == d.textAtom {
			typ = d.utf8String
		}
		if len(d.currentData) > x11ChunkSize {
			// Too large for one request, so it goes over in pieces:
			// announce the size, watch the requestor delete the
			// property, and send the next piece each time it does.
			var b [4]byte
			xgb.Put32(b[:], uint32(len(d.currentData)))
			xproto.ChangeProperty(d.conn, xproto.PropModeReplace, e.Requestor, prop,
				d.incr, 32, 1, b[:])
			xproto.ChangeWindowAttributes(d.conn, e.Requestor, xproto.CwEventMask,
				[]uint32{uint32(xproto.EventMaskPropertyChange)})
			d.sending[e.Requestor] = &x11OutgoingTransfer{prop: prop, rest: d.currentData}
		} else {
			xproto.ChangeProperty(d.conn, xproto.PropModeReplace, e.Requestor, prop,
				typ, 8, uint32(len(d.currentData)), []byte(d.currentData))
		}

	default:
		d.notify(e, xproto.AtomNone)
		return
	}

	d.notify(e, prop)
}

// sendNextChunk hands over one more piece of an incremental transfer. A piece
// of length zero is how the end of one is announced.
func (d *X11Driver) sendNextChunk(requestor xproto.Window) {
	d.mu.Lock()
	defer d.mu.Unlock()
	xfer, ok := d.sending[requestor]
	if !ok || d.conn == nil {
		return
	}

	n := x11ChunkSize
	if n > len(xfer.rest) {
		n = len(xfer.rest)
	}
	xproto.ChangeProperty(d.conn, xproto.PropModeReplace, requestor, xfer.prop,
		d.utf8String, 8, uint32(n), []byte(xfer.rest[:n]))
	xfer.rest = xfer.rest[n:]
	if n == 0 {
		delete(d.sending, requestor)
	}
}

func (d *X11Driver) notify(e xproto.SelectionRequestEvent, prop xproto.Atom) {
	if d.conn == nil {
		return
	}
	ev := xproto.SelectionNotifyEvent{
		Time:      e.Time,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  prop,
	}
	xproto.SendEvent(d.conn, false, e.Requestor, 0, string(ev.Bytes()))
}
