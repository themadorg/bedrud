package tray

import (
	"fmt"
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

const (
	sniIface     = "org.kde.StatusNotifierItem"
	watcherKDE   = "org.kde.StatusNotifierWatcher"
	watcherAyat  = "org.ayatana.StatusNotifierWatcher"
	watcherIface = "org.kde.StatusNotifierWatcher"
	menuIface    = "com.canonical.dbusmenu"
	sniPath      = "/StatusNotifierItem"
	menuPath     = "/MenuBar"
)

type Callbacks struct {
	Toggle  func()
	Create  func()
	Quit    func()
}

type Host struct {
	conn       *dbus.Conn
	props      *prop.Properties
	menu       *menuServer
	cb         Callbacks
	xembedStop func()
}

type sniServer struct {
	h *Host
}

func (s *sniServer) Activate(x, y int32) *dbus.Error {
	_ = x
	_ = y
	if s.h.cb.Toggle != nil {
		s.h.cb.Toggle()
	}
	return nil
}

func (s *sniServer) SecondaryActivate(x, y int32) *dbus.Error {
	return s.Activate(x, y)
}

func (s *sniServer) ContextMenu(x, y int32) *dbus.Error {
	_ = x
	_ = y
	return nil
}

func (s *sniServer) Scroll(delta int32, orientation string) *dbus.Error {
	_ = delta
	_ = orientation
	return nil
}

type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

type menuServer struct {
	h        *Host
	revision uint32
}

func (m *menuServer) GetLayout(parentID, recursionDepth int32, propertyNames []string) (uint32, menuLayout, *dbus.Error) {
	_ = recursionDepth
	_ = propertyNames
	if parentID != 0 {
		return m.revision, menuLayout{ID: parentID, Properties: map[string]dbus.Variant{}}, nil
	}
	return m.revision, menuLayout{
		ID: 0,
		Properties: map[string]dbus.Variant{
			"children-display": dbus.MakeVariant("submenu"),
		},
		Children: []dbus.Variant{
			dbus.MakeVariant(item(1, "standard", "Show window")),
			dbus.MakeVariant(item(2, "standard", "Create host")),
			dbus.MakeVariant(item(3, "separator", "")),
			dbus.MakeVariant(item(4, "standard", "Quit")),
		},
	}, nil
}

func item(id int32, typ, label string) menuLayout {
	p := map[string]dbus.Variant{
		"type": dbus.MakeVariant(typ),
	}
	if label != "" {
		p["label"] = dbus.MakeVariant(label)
		p["enabled"] = dbus.MakeVariant(true)
		p["visible"] = dbus.MakeVariant(true)
	}
	return menuLayout{ID: id, Properties: p, Children: []dbus.Variant{}}
}

func (m *menuServer) GetGroupProperties(ids []int32, propertyNames []string) ([]struct {
	ID  int32
	Props map[string]dbus.Variant
}, *dbus.Error) {
	_ = ids
	_ = propertyNames
	return nil, nil
}

func (m *menuServer) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	_ = id
	_ = name
	return dbus.MakeVariant(""), nil
}

func (m *menuServer) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	_ = data
	_ = timestamp
	if eventID != "clicked" {
		return nil
	}
	switch id {
	case 1:
		if m.h.cb.Toggle != nil {
			m.h.cb.Toggle()
		}
	case 2:
		if m.h.cb.Create != nil {
			m.h.cb.Create()
		}
	case 4:
		if m.h.cb.Quit != nil {
			m.h.cb.Quit()
		}
	}
	return nil
}

func (m *menuServer) AboutToShow(id int32) (bool, *dbus.Error) {
	_ = id
	return false, nil
}

func Start(cb Callbacks) (*Host, error) {
	if stop, err := startXEmbed(cb); err == nil {
		return &Host{cb: cb, xembedStop: stop}, nil
	} else {
		log.Printf("xembed tray: %v", err)
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	h := &Host{conn: conn, cb: cb}

	sni := &sniServer{h: h}
	if err := conn.Export(sni, sniPath, sniIface); err != nil {
		_ = conn.Close()
		return nil, err
	}

	tooltip := struct {
		IconName   string
		IconPixmap []struct {
			W, H int32
			D    []byte
		}
		Title string
		Body  string
	}{
		IconName: "network-server",
		Title:    "Bedrud Host",
		Body:     "Idle",
	}

	propsSpec := map[string]map[string]*prop.Prop{
		sniIface: {
			"Category":    {Value: "ApplicationStatus", Writable: false, Emit: prop.EmitTrue},
			"Id":          {Value: "org.bedrud.HostGui", Writable: false, Emit: prop.EmitTrue},
			"Title":       {Value: "Bedrud Host", Writable: false, Emit: prop.EmitTrue},
			"Status":      {Value: "Active", Writable: false, Emit: prop.EmitTrue},
			"WindowId":    {Value: uint32(0), Writable: false, Emit: prop.EmitTrue},
			"IconName":    {Value: "network-server", Writable: false, Emit: prop.EmitTrue},
			"OverlayIconName": {Value: "", Writable: false, Emit: prop.EmitTrue},
			"AttentionIconName": {Value: "", Writable: false, Emit: prop.EmitTrue},
			"AttentionMovieName": {Value: "", Writable: false, Emit: prop.EmitTrue},
			"ToolTip":     {Value: tooltip, Writable: false, Emit: prop.EmitTrue},
			"ItemIsMenu":  {Value: false, Writable: false, Emit: prop.EmitTrue},
			"Menu":        {Value: dbus.ObjectPath(menuPath), Writable: false, Emit: prop.EmitTrue},
			"IconThemePath": {Value: "", Writable: false, Emit: prop.EmitTrue},
		},
	}
	props, err := prop.Export(conn, sniPath, propsSpec)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	h.props = props

	h.menu = &menuServer{h: h, revision: 1}
	if err := conn.Export(h.menu, menuPath, menuIface); err != nil {
		_ = conn.Close()
		return nil, err
	}

	sniNode := introspect.Node{
		Name: sniPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:    sniIface,
				Methods: introspect.Methods(sni),
			},
		},
	}
	_ = conn.Export(introspect.NewIntrospectable(&sniNode), sniPath, "org.freedesktop.DBus.Introspectable")

	menuNode := introspect.Node{
		Name: menuPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			{Name: menuIface, Methods: introspect.Methods(h.menu)},
		},
	}
	_ = conn.Export(introspect.NewIntrospectable(&menuNode), menuPath, "org.freedesktop.DBus.Introspectable")

	if err := registerWatcher(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return h, nil
}

func registerWatcher(conn *dbus.Conn) error {
	id := conn.Names()[0]
	var last error
	for _, dest := range []string{watcherKDE, watcherAyat} {
		obj := conn.Object(dest, "/StatusNotifierWatcher")
		call := obj.Call(watcherIface+".RegisterStatusNotifierItem", 0, id)
		if call.Err == nil {
			return nil
		}
		last = call.Err
	}
	return fmt.Errorf("status notifier watcher: %w", last)
}

func (h *Host) SetStatus(title, body, sniStatus string) {
	if h == nil || h.props == nil {
		return
	}
	if sniStatus == "" {
		sniStatus = "Active"
	}
	tooltip := struct {
		IconName   string
		IconPixmap []struct {
			W, H int32
			D    []byte
		}
		Title string
		Body  string
	}{
		IconName: "network-server",
		Title:    title,
		Body:     body,
	}
	if err := h.props.Set(sniIface, "ToolTip", dbus.MakeVariant(tooltip)); err != nil {
		log.Printf("tray tooltip: %v", err)
	}
	if err := h.props.Set(sniIface, "Status", dbus.MakeVariant(sniStatus)); err != nil {
		log.Printf("tray status: %v", err)
	}
}

func (h *Host) Close() {
	if h == nil {
		return
	}
	if h.xembedStop != nil {
		h.xembedStop()
	}
	if h.conn != nil {
		_ = h.conn.Close()
	}
}
