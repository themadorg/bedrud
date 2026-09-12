package ui

import (
	_ "embed"
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed icons/linode.svg
var linodeSVG []byte

//go:embed icons/cloudflare.svg
var cloudflareSVG []byte

func svgPaintable(data []byte, px int) gdk.Paintabler {
	if px < 1 {
		px = 128
	}
	ld, err := gdkpixbuf.NewPixbufLoaderWithMIMEType("image/svg+xml")
	if err != nil {
		log.Printf("svg loader: %v", err)
		return nil
	}
	ld.SetSize(px, px)
	if err := ld.Write(data); err != nil {
		log.Printf("svg write: %v", err)
		_ = ld.Close()
		return nil
	}
	if err := ld.Close(); err != nil {
		log.Printf("svg close: %v", err)
		return nil
	}
	pb := ld.Pixbuf()
	if pb == nil {
		return nil
	}
	return gdk.NewTextureForPixbuf(pb)
}

func svgImage(data []byte, px int) *gtk.Image {
	img := gtk.NewImage()
	img.SetPixelSize(px)
	if p := svgPaintable(data, px); p != nil {
		img.SetFromPaintable(p)
	}
	img.SetHAlign(gtk.AlignCenter)
	return img
}
