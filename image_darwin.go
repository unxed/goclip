//go:build darwin

package goclip

import (
	"context"
	"encoding/base64"
	"image"
	"strings"
)

// The system JXA bridge talks to NSPasteboard/AppKit without linking CGO.
const pasteboardImageScript = `ObjC.import('AppKit');
var p=$.NSPasteboard.generalPasteboard;
var data=p.dataForType('public.png');
if (!data) {
 var t=p.dataForType('public.tiff');
 if (t) {
  var rep=$.NSBitmapImageRep.imageRepWithData(t);
  data=rep.representationUsingTypeProperties($.NSBitmapImageFileTypePNG,$());
 }
}
data ? ObjC.unwrap(data.base64EncodedStringWithOptions(0)) : '';`

func (d *DarwinDriver) ReadImage(ctx context.Context) (image.Image, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	encoded, err := imageCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", pasteboardImageScript)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(encoded))) == 0 {
		return nil, ErrNoImage
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		return nil, err
	}
	return decodeClipboardImage(data)
}
