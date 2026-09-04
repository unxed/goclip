//go:build !android

package goclip

// These run against a real X server, which on a machine without one is no
// server at all:
//
//	Xvfb :99 -screen 0 800x600x24 &
//	DISPLAY=:99 go test ./...
//
// Without $DISPLAY they skip.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

func needX(t *testing.T) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
}

// The native driver has to work at all. It did not: the window it created was
// InputOnly with a non-zero depth, which is a BadMatch, so every call failed
// and fell through to xclip.
func TestX11DriverConnects(t *testing.T) {
	needX(t)
	d := NewX11Driver()
	if err := d.ensure(); err != nil {
		t.Fatalf("the native X11 driver could not start: %v", err)
	}
	if !d.Available() {
		t.Error("a driver that connected is available")
	}
}

func TestX11RoundTrip(t *testing.T) {
	needX(t)
	writer, reader := NewX11Driver(), NewX11Driver()

	const text = "путь/к/файлу с пробелом.txt"
	if err := writer.WriteText(text); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := reader.ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != text {
		t.Errorf("clipboard: got %q, want %q", got, text)
	}
}

// Reading back what we ourselves are offering must not go through the server:
// answering our own request would need the event loop to be in two places.
func TestX11ReadsBackItsOwnOffer(t *testing.T) {
	needX(t)
	d := NewX11Driver()
	if err := d.WriteText("mine"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got, err := d.ReadText(); err != nil || got != "mine" {
		t.Errorf("got %q, %v", got, err)
	}
}

// Anything past a couple of hundred kilobytes does not fit in one X request
// and has to be handed over in pieces. Both ends of that are exercised here:
// this driver sends the pieces and the other one puts them back together.
func TestX11LargeRoundTrip(t *testing.T) {
	needX(t)
	writer, reader := NewX11Driver(), NewX11Driver()

	want := strings.Repeat("сорок два.", 40000)
	if err := writer.WriteText(want); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := reader.ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != want {
		t.Errorf("large clipboard came back as %d bytes, want %d", len(got), len(want))
	}
}

// An empty clipboard is empty, not an error in disguise.
func TestX11EmptyClipboard(t *testing.T) {
	needX(t)
	d := NewX11Driver()
	if err := d.WriteText(""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := d.ReadText(); err != ErrEmpty {
		t.Errorf("an empty clipboard reads as ErrEmpty, got %v", err)
	}
}

// TARGETS is what a well behaved application asks first, to find out which
// formats are on offer.
func TestX11AnswersTargets(t *testing.T) {
	needX(t)
	owner := NewX11Driver()
	if err := owner.WriteText("anything"); err != nil {
		t.Fatalf("copy: %v", err)
	}

	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("second connection: %v", err)
	}
	defer conn.Close()
	screen := xproto.Setup(conn).DefaultScreen(conn)
	win, err := xproto.NewWindowId(conn)
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	err = xproto.CreateWindowChecked(conn, screen.RootDepth, win, screen.Root,
		0, 0, 1, 1, 0, xproto.WindowClassInputOutput, screen.RootVisual,
		xproto.CwEventMask, []uint32{uint32(xproto.EventMaskPropertyChange)}).Check()
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	at := func(n string) xproto.Atom {
		r, err := xproto.InternAtom(conn, false, uint16(len(n)), n).Reply()
		if err != nil || r == nil {
			t.Fatalf("intern %s: %v", n, err)
		}
		return r.Atom
	}
	sel, targets, prop, utf8 := at("CLIPBOARD"), at("TARGETS"), at("TEST_PROP"), at("UTF8_STRING")

	xproto.ConvertSelection(conn, win, sel, targets, prop, xproto.TimeCurrentTime)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("no answer to TARGETS")
		}
		ev, _ := conn.WaitForEvent()
		if ev == nil {
			continue
		}
		sn, ok := ev.(xproto.SelectionNotifyEvent)
		if !ok {
			continue
		}
		if sn.Property == xproto.AtomNone {
			t.Fatal("TARGETS was refused")
		}
		reply, err := xproto.GetProperty(conn, false, win, prop,
			xproto.GetPropertyTypeAny, 0, 64).Reply()
		if err != nil || reply == nil {
			t.Fatalf("read TARGETS: %v", err)
		}
		found := false
		for i := 0; i+4 <= len(reply.Value); i += 4 {
			if xproto.Atom(xgb.Get32(reply.Value[i:])) == utf8 {
				found = true
			}
		}
		if !found {
			t.Error("UTF8_STRING must be among the targets offered")
		}
		return
	}
}

// A request for something we do not have must be refused rather than answered
// with the wrong thing.
func TestX11RefusesUnknownTarget(t *testing.T) {
	needX(t)
	owner := NewX11Driver()
	if err := owner.WriteText("anything"); err != nil {
		t.Fatalf("copy: %v", err)
	}

	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("second connection: %v", err)
	}
	defer conn.Close()
	screen := xproto.Setup(conn).DefaultScreen(conn)
	win, err := xproto.NewWindowId(conn)
	if err != nil {
		t.Fatalf("window: %v", err)
	}
	if err := xproto.CreateWindowChecked(conn, screen.RootDepth, win, screen.Root,
		0, 0, 1, 1, 0, xproto.WindowClassInputOutput, screen.RootVisual,
		0, nil).Check(); err != nil {
		t.Fatalf("window: %v", err)
	}
	at := func(n string) xproto.Atom {
		r, _ := xproto.InternAtom(conn, false, uint16(len(n)), n).Reply()
		return r.Atom
	}

	xproto.ConvertSelection(conn, win, at("CLIPBOARD"), at("image/png"),
		at("TEST_PROP2"), xproto.TimeCurrentTime)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("no answer at all")
		}
		ev, _ := conn.WaitForEvent()
		if sn, ok := ev.(xproto.SelectionNotifyEvent); ok {
			if sn.Property != xproto.AtomNone {
				t.Error("a target we cannot supply must be refused")
			}
			return
		}
	}
}

// The value has to come from whoever owns the selection now, not from a stale
// copy of what we last put there.
func TestX11FollowsANewOwner(t *testing.T) {
	needX(t)
	first, second, reader := NewX11Driver(), NewX11Driver(), NewX11Driver()

	if err := first.WriteText("older"); err != nil {
		t.Fatalf("first copy: %v", err)
	}
	if err := second.WriteText("newer"); err != nil {
		t.Fatalf("second copy: %v", err)
	}
	got, err := reader.ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != "newer" {
		t.Errorf("got %q from the old owner, want the new one", got)
	}
}
