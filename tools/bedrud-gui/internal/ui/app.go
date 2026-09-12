package ui

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"bedrud-gui/internal/hostcli"
	"bedrud-gui/internal/tray"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
)

const css = `
.hosts-flow {
  padding: 18px;
}
.host-card {
  min-width: 280px;
  padding: 14px;
}
.host-card-title {
  font-weight: 600;
}
.host-card-creating {
  min-width: 300px;
}
flowboxchild:selected .host-card {
  outline: 2px solid @accent_bg_color;
  outline-offset: 2px;
}
`

type App struct {
	adw *adw.Application
	win *adw.ApplicationWindow

	toast   *adw.ToastOverlay
	stack   *gtk.Stack
	header   *adw.HeaderBar
	addBtn   *gtk.Button
	refresh  *gtk.Button
	gearBtn  *gtk.Button
	backBtn  *gtk.Button
	pageName string

	setDomain *adw.ActionRow
	setHosts  *adw.ActionRow
	setAvg    *adw.ActionRow
	setDir    *adw.ActionRow
	setBin    *adw.ActionRow
	setDB     *adw.ActionRow

	opBtn    *gtk.Button
	opStack  *gtk.Stack
	opSpin   *gtk.Spinner
	opIcon   *gtk.Image
	opPop    *gtk.Popover
	opTitle  *gtk.Label
	opStatus *gtk.Label
	opDetail *gtk.Label
	opStart  time.Time
	opTick   glib.SourceHandle
	opBusy   bool

	flow      *gtk.FlowBox
	scroller  *gtk.ScrolledWindow
	emptyPage *adw.StatusPage
	lastHosts []hostcli.Host

	bin       string
	tray      *tray.Host
	pending     *createCard
	createAvg   time.Duration
	deletingID  int
	deleteStage string

	initStack *gtk.Stack
	initIndex int
	initDots  []*gtk.Label
	initBack  *gtk.Button
	initNext  *gtk.Button
	initLinode   *adw.PasswordEntryRow
	initCFTok    *adw.PasswordEntryRow
	initCFEmail  *adw.EntryRow
	initCFKey    *adw.PasswordEntryRow
	initDomain   *adw.EntryRow
	initZone     *adw.EntryRow
	logoLinode   gdk.Paintabler
	logoCF       gdk.Paintabler
}

type createCard struct {
	widget *gtk.Box
	title  *gtk.Label
	meta   *gtk.Label
	eta    *gtk.Label
	bar    *gtk.ProgressBar
	spin   *gtk.Spinner
	log    strings.Builder
	start  time.Time
	avg    time.Duration
	prog   hostcli.CreateProgress
	tick   glib.SourceHandle
	failed bool
	dlg    *adw.Dialog
	dlgTitle *gtk.Label
	dlgMeta  *gtk.Label
	dlgEta   *gtk.Label
	dlgBar   *gtk.ProgressBar
	dlgBuf   *gtk.TextBuffer
}

func Attach(application *adw.Application) {
	a := &App{adw: application}
	application.ConnectActivate(a.activate)
}

func (a *App) activate() {
	if a.win != nil {
		a.win.Present()
		return
	}

	cssp := gtk.NewCSSProvider()
	cssp.LoadFromString(css)
	gtk.StyleContextAddProviderForDisplay(
		gdk.DisplayGetDefault(),
		cssp,
		gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
	)

	a.win = adw.NewApplicationWindow(&a.adw.Application)
	a.win.SetTitle("Bedrud Host")
	a.win.SetDefaultSize(880, 640)

	a.toast = adw.NewToastOverlay()
	toolbar := adw.NewToolbarView()
	a.header = adw.NewHeaderBar()

	a.backBtn = gtk.NewButtonFromIconName("go-previous-symbolic")
	a.backBtn.SetTooltipText("Back")
	a.backBtn.SetVisible(false)
	a.backBtn.ConnectClicked(a.closeSettings)
	a.header.PackStart(a.backBtn)

	a.gearBtn = gtk.NewButtonFromIconName("preferences-system-symbolic")
	a.gearBtn.SetTooltipText("Settings")
	a.gearBtn.AddCSSClass("flat")
	a.gearBtn.ConnectClicked(a.openSettings)
	a.header.PackStart(a.gearBtn)

	a.addBtn = gtk.NewButtonFromIconName("list-add-symbolic")
	a.addBtn.SetTooltipText("Create host")
	a.addBtn.AddCSSClass("suggested-action")
	a.addBtn.ConnectClicked(a.onCreate)
	a.header.PackStart(a.addBtn)

	a.refresh = gtk.NewButtonFromIconName("view-refresh-symbolic")
	a.refresh.SetTooltipText("Reload list")
	a.refresh.ConnectClicked(func() { a.reloadHosts() })
	a.header.PackStart(a.refresh)

	a.header.PackEnd(a.buildOpButton())

	toolbar.AddTopBar(a.header)

	a.stack = gtk.NewStack()
	a.stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	a.stack.AddNamed(a.buildLoadingPage(), "loading")
	a.stack.AddNamed(a.buildInitPage(), "init")
	a.stack.AddNamed(a.buildHostsPage(), "hosts")
	a.stack.AddNamed(a.buildSettingsPage(), "settings")
	a.stack.AddNamed(a.buildMissingBinPage(), "nobin")
	a.addBtn.SetVisible(false)
	a.refresh.SetVisible(false)

	toolbar.SetContent(a.stack)
	a.toast.SetChild(toolbar)
	a.win.SetContent(a.toast)
	a.win.ConnectCloseRequest(func() bool {
		if a.tray != nil {
			a.tray.Close()
			a.tray = nil
		}
		return false
	})
	a.win.Present()

	a.startTray()
	a.setChromeBusy(false)
	a.boot()
}

func (a *App) startTray() {
	h, err := tray.Start(tray.Callbacks{
		Toggle: func() {
			glib.IdleAdd(func() { a.toggleWindow() })
		},
		Create: func() {
			glib.IdleAdd(func() {
				a.win.Present()
				a.onCreate()
			})
		},
		Quit: func() {
			glib.IdleAdd(func() {
				if a.tray != nil {
					a.tray.Close()
				}
				a.adw.Quit()
			})
		},
	})
	if err != nil {
		log.Printf("system tray: %v", err)
		a.toastMsg("No system tray (need polybar tray or StatusNotifier)")
		return
	}
	a.tray = h
}

func (a *App) toggleWindow() {
	if a.win == nil {
		return
	}
	if a.win.IsVisible() {
		a.win.SetVisible(false)
		return
	}
	a.win.Present()
}

func (a *App) trayStatus(sni string) {
	if a.tray == nil {
		return
	}
	title := "Bedrud Host"
	if a.opTitle != nil {
		title = a.opTitle.Text()
	}
	body := "Idle"
	if a.opStatus != nil {
		body = a.opStatus.Text()
	}
	a.tray.SetStatus(title, body, sni)
}

func (a *App) toastMsg(msg string) {
	t := adw.NewToast(msg)
	t.SetTimeout(4)
	a.toast.AddToast(t)
}

func (a *App) buildOpButton() gtk.Widgetter {
	a.opTitle = gtk.NewLabel("No operation")
	a.opTitle.AddCSSClass("heading")
	a.opTitle.SetXAlign(0)
	a.opTitle.SetWrap(true)

	a.opStatus = gtk.NewLabel("Idle")
	a.opStatus.AddCSSClass("dim-label")
	a.opStatus.SetXAlign(0)

	a.opDetail = gtk.NewLabel("")
	a.opDetail.AddCSSClass("dim-label")
	a.opDetail.SetXAlign(0)
	a.opDetail.SetWrap(true)
	a.opDetail.SetVisible(false)
	a.opDetail.SetMaxWidthChars(36)

	box := gtk.NewBox(gtk.OrientationVertical, 4)
	box.SetMarginStart(12)
	box.SetMarginEnd(12)
	box.SetMarginTop(10)
	box.SetMarginBottom(10)
	box.Append(a.opTitle)
	box.Append(a.opStatus)
	box.Append(a.opDetail)

	a.opPop = gtk.NewPopover()
	a.opPop.SetChild(box)
	a.opPop.SetAutohide(true)

	a.opSpin = gtk.NewSpinner()
	a.opIcon = gtk.NewImageFromIconName("dialog-error-symbolic")
	a.opStack = gtk.NewStack()
	a.opStack.AddNamed(a.opSpin, "busy")
	a.opStack.AddNamed(a.opIcon, "idle")
	a.opStack.SetVisibleChildName("busy")

	a.opBtn = gtk.NewButton()
	a.opBtn.SetChild(a.opStack)
	a.opBtn.AddCSSClass("flat")
	a.opBtn.SetTooltipText("Operation status")
	a.opBtn.SetVisible(false)
	a.opPop.SetParent(a.opBtn)
	a.opBtn.ConnectClicked(func() {
		a.opPop.Popup()
	})
	return a.opBtn
}

func (a *App) setChromeBusy(busy bool) {
	a.opBusy = busy
	if a.addBtn != nil {
		a.addBtn.SetSensitive(!busy)
	}
	if a.refresh != nil {
		a.refresh.SetSensitive(!busy)
	}
}

func (a *App) beginOp(title string) {
	a.setChromeBusy(true)
	a.opStart = time.Now()
	a.opTitle.SetText(title)
	a.opStatus.SetText("Running")
	a.opDetail.SetText("")
	a.opDetail.SetVisible(false)
	a.opSpin.Start()
	a.opStack.SetVisibleChildName("busy")
	a.opBtn.SetVisible(true)
	a.opBtn.SetTooltipText(title + " — running")
	if a.opTick != 0 {
		glib.SourceRemove(a.opTick)
		a.opTick = 0
	}
	a.opTick = glib.TimeoutAdd(1000, func() bool {
		if !a.opBusy {
			return false
		}
		a.opStatus.SetText("Running · " + time.Since(a.opStart).Truncate(time.Second).String())
		a.opBtn.SetTooltipText(a.opTitle.Text() + " — " + a.opStatus.Text())
		return true
	})
	a.trayStatus("NeedsAttention")
}

func (a *App) endOp(err error) {
	if a.opTick != 0 {
		glib.SourceRemove(a.opTick)
		a.opTick = 0
	}
	a.setChromeBusy(false)
	a.opSpin.Stop()
	elapsed := time.Since(a.opStart).Truncate(time.Second)
	if elapsed < time.Second {
		elapsed = time.Second
	}
	if err != nil {
		a.opStatus.SetText("Failed · " + elapsed.String())
		a.opDetail.SetText(err.Error())
		a.opDetail.SetVisible(true)
		a.opIcon.SetFromIconName("dialog-error-symbolic")
		a.opStack.SetVisibleChildName("idle")
		a.opBtn.SetVisible(true)
		a.opBtn.SetTooltipText(a.opTitle.Text() + " — failed")
	} else {
		a.opStatus.SetText("Finished · " + elapsed.String())
		a.opDetail.SetText("")
		a.opDetail.SetVisible(false)
		a.opBtn.SetVisible(false)
		a.opBtn.SetTooltipText(a.opTitle.Text() + " — finished")
	}
	if err != nil {
		a.trayStatus("NeedsAttention")
	} else {
		a.trayStatus("Active")
	}
}

func (a *App) boot() {
	bin, err := hostcli.FindBin()
	if err != nil {
		a.showPage("nobin")
		return
	}
	a.bin = bin
	a.showPage("loading")
	go func() {
		st, err := hostcli.StatusCmdLocal(bin, true)
		if err != nil && !st.Initialized {
			log.Printf("status: %v", err)
		}
		if !st.Initialized {
			glib.IdleAdd(func() {
				a.showPage("init")
			})
			return
		}
		hosts, _, lerr := hostcli.List(bin)
		glib.IdleAdd(func() {
			if st.CreateAvg > 0 {
				a.createAvg = st.CreateAvg
			}
			a.showHostsReady(hosts, lerr)
		})
	}()
}

func (a *App) showHostsReady(hosts []hostcli.Host, err error) {
	a.showPage("hosts")
	if err != nil {
		a.toastMsg(err.Error())
		return
	}
	a.renderHosts(hosts)
}

func (a *App) showHosts() {
	a.showPage("hosts")
	a.reloadHosts()
}

func (a *App) showPage(name string) {
	a.pageName = name
	a.stack.SetVisibleChildName(name)
	settings := name == "settings"
	a.backBtn.SetVisible(settings)
	a.gearBtn.SetVisible(!settings && name != "nobin")
	a.addBtn.SetVisible(name == "hosts")
	a.refresh.SetVisible(name == "hosts")
	if settings {
		a.header.SetTitleWidget(gtk.NewLabel("Settings"))
	} else if name == "init" {
		a.header.SetTitleWidget(gtk.NewLabel("Initialize"))
		a.setInitPage(0)
	} else {
		a.header.SetTitleWidget(nil)
		a.win.SetTitle("Bedrud Host")
	}
}

func (a *App) buildSettingsPage() gtk.Widgetter {
	page := adw.NewPreferencesPage()
	page.SetName("settings")

	info := adw.NewPreferencesGroup()
	info.SetTitle("Local setup")
	info.SetDescription("Credentials and hosts live in an encrypted SQLite file. This does not delete cloud VMs.")

	a.setDomain = adw.NewActionRow()
	a.setDomain.SetTitle("DNS zone")
	a.setDomain.SetSubtitleSelectable(true)
	info.Add(a.setDomain)

	a.setHosts = adw.NewActionRow()
	a.setHosts.SetTitle("Saved hosts")
	info.Add(a.setHosts)

	a.setAvg = adw.NewActionRow()
	a.setAvg.SetTitle("Average create time")
	info.Add(a.setAvg)

	a.setDir = adw.NewActionRow()
	a.setDir.SetTitle("Config directory")
	a.setDir.SetSubtitleSelectable(true)
	info.Add(a.setDir)

	a.setDB = adw.NewActionRow()
	a.setDB.SetTitle("Database")
	a.setDB.SetSubtitleSelectable(true)
	info.Add(a.setDB)

	a.setBin = adw.NewActionRow()
	a.setBin.SetTitle("CLI binary")
	a.setBin.SetSubtitleSelectable(true)
	info.Add(a.setBin)
	page.Add(info)

	acts := adw.NewPreferencesGroup()
	acts.SetTitle("Actions")

	openRow := adw.NewActionRow()
	openRow.SetTitle("Open config folder")
	openBtn := gtk.NewButtonFromIconName("folder-open-symbolic")
	openBtn.SetVAlign(gtk.AlignCenter)
	openBtn.AddCSSClass("flat")
	openBtn.ConnectClicked(func() {
		p := a.setDir.Subtitle()
		if p == "" {
			return
		}
		gtk.ShowURI(&a.win.Window, "file://"+p, gdk.CURRENT_TIME)
	})
	openRow.AddSuffix(openBtn)
	openRow.SetActivatableWidget(openBtn)
	acts.Add(openRow)

	credRow := adw.NewActionRow()
	credRow.SetTitle("Edit credentials")
	credRow.SetSubtitle("Opens the initialize form. Existing hosts stay until you delete the database.")
	credBtn := gtk.NewButtonWithLabel("Edit")
	credBtn.SetVAlign(gtk.AlignCenter)
	credBtn.ConnectClicked(func() { a.showPage("init") })
	credRow.AddSuffix(credBtn)
	credRow.SetActivatableWidget(credBtn)
	acts.Add(credRow)
	page.Add(acts)

	danger := adw.NewPreferencesGroup()
	danger.SetTitle("Danger zone")
	danger.SetDescription("Removes the local encrypted database and credentials. Linode VMs and DNS records are not deleted.")

	delRow := adw.NewActionRow()
	delRow.SetTitle("Delete local database")
	delBtn := gtk.NewButtonWithLabel("Delete")
	delBtn.AddCSSClass("destructive-action")
	delBtn.SetVAlign(gtk.AlignCenter)
	delBtn.ConnectClicked(a.confirmResetDB)
	delRow.AddSuffix(delBtn)
	danger.Add(delRow)
	page.Add(danger)

	sc := gtk.NewScrolledWindow()
	sc.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	sc.SetChild(page)
	return sc
}

func (a *App) openSettings() {
	a.showPage("settings")
	a.refreshSettings()
}

func (a *App) closeSettings() {
	if a.bin == "" {
		a.showPage("nobin")
		return
	}
	go func() {
		st, err := hostcli.StatusCmdLocal(a.bin, true)
		glib.IdleAdd(func() {
			if err != nil && !st.Initialized {
				a.showPage("init")
				return
			}
			if st.Initialized {
				a.showHosts()
				return
			}
			a.showPage("init")
		})
	}()
}

func (a *App) refreshSettings() {
	if a.bin == "" {
		return
	}
	go func() {
		st, err := hostcli.StatusCmdLocal(a.bin, true)
		glib.IdleAdd(func() {
			if err != nil {
				log.Printf("settings status: %v", err)
			}
			set := func(row *adw.ActionRow, key, fallback string) {
				v := st.Lines[key]
				if v == "" {
					v = fallback
				}
				row.SetSubtitle(v)
			}
			set(a.setDomain, "cloudflare-domain", "not set")
			set(a.setHosts, "local-hosts", "0")
			set(a.setAvg, "create-avg", "no samples yet")
			set(a.setDir, "config-dir", "")
			set(a.setDB, "encrypted-db", "")
			set(a.setBin, "binary", a.bin)
		})
	}()
}

func (a *App) confirmResetDB() {
	d := adw.NewAlertDialog("Delete local database?", "This removes saved credentials and the host list on this computer. Cloud servers are not deleted.")
	d.AddResponse("cancel", "Cancel")
	d.AddResponse("delete", "Delete")
	d.SetResponseAppearance("delete", adw.ResponseDestructive)
	d.SetDefaultResponse("cancel")
	d.SetCloseResponse("cancel")
	d.ConnectResponse(func(response string) {
		if response != "delete" {
			return
		}
		a.beginOp("Delete database")
		go func() {
			_, err := hostcli.Reset(a.bin)
			glib.IdleAdd(func() {
				a.endOp(err)
				if err != nil {
					a.toastMsg(err.Error())
					return
				}
				a.toastMsg("Local database deleted")
				a.renderHosts(nil)
				a.showPage("init")
			})
		}()
	})
	d.Present(a.win)
}

func (a *App) buildLoadingPage() gtk.Widgetter {
	p := adw.NewStatusPage()
	p.SetIconName("network-server-symbolic")
	p.SetTitle("Opening")
	p.SetDescription("Loading saved setup…")
	spin := gtk.NewSpinner()
	spin.SetHAlign(gtk.AlignCenter)
	spin.Start()
	p.SetChild(spin)
	return p
}

func (a *App) buildMissingBinPage() gtk.Widgetter {
	p := adw.NewStatusPage()
	p.SetIconName("dialog-error-symbolic")
	p.SetTitle("bedrud-host not found")
	p.SetDescription("Build the CLI first, or set BEDRUD_HOST_BIN to the binary path.")
	return p
}

func (a *App) buildInitPage() gtk.Widgetter {
	a.initStack = gtk.NewStack()
	a.initStack.SetTransitionType(gtk.StackTransitionTypeSlideLeftRight)
	a.initStack.SetVExpand(true)
	a.initStack.SetHExpand(true)
	pages := []gtk.Widgetter{a.initWelcomePage(), a.initLinodePage(), a.initCloudflarePage()}
	for i, p := range pages {
		gtk.BaseWidget(p).SetHExpand(true)
		gtk.BaseWidget(p).SetVExpand(true)
		sc := gtk.NewScrolledWindow()
		sc.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
		sc.SetHExpand(true)
		sc.SetVExpand(true)
		sc.SetChild(p)
		a.initStack.AddNamed(sc, strconv.Itoa(i))
	}

	dots := gtk.NewBox(gtk.OrientationHorizontal, 8)
	dots.SetHAlign(gtk.AlignCenter)
	a.initDots = nil
	for range pages {
		d := gtk.NewLabel("•")
		d.AddCSSClass("title-2")
		dots.Append(d)
		a.initDots = append(a.initDots, d)
	}

	a.initBack = gtk.NewButtonWithLabel("Back")
	a.initBack.ConnectClicked(func() {
		if a.initIndex > 0 {
			a.setInitPage(a.initIndex - 1)
		}
	})
	a.initNext = gtk.NewButtonWithLabel("Continue")
	a.initNext.AddCSSClass("suggested-action")
	a.initNext.AddCSSClass("pill")
	a.initNext.ConnectClicked(a.initNextClicked)

	nav := gtk.NewBox(gtk.OrientationHorizontal, 12)
	nav.SetHAlign(gtk.AlignCenter)
	nav.SetMarginTop(8)
	nav.SetMarginBottom(18)
	nav.Append(a.initBack)
	nav.Append(a.initNext)

	outer := gtk.NewBox(gtk.OrientationVertical, 8)
	outer.SetHExpand(true)
	outer.SetVExpand(true)
	outer.Append(a.initStack)
	outer.Append(dots)
	outer.Append(nav)
	a.setInitPage(0)
	return outer
}

func (a *App) initWelcomePage() gtk.Widgetter {
	p := adw.NewStatusPage()
	p.SetIconName("network-server-symbolic")
	p.SetTitle("Create Bedrud servers when you need them")

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(520)
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginStart(24)
	box.SetMarginEnd(24)

	lead := gtk.NewLabel("This app creates and manages Bedrud meeting servers. Spawn a host whenever you need one, then view, open, or delete it from cards on the home screen.")
	lead.SetWrap(true)
	lead.SetJustify(gtk.JustifyCenter)
	lead.SetXAlign(0.5)
	lead.SetMaxWidthChars(48)
	lead.AddCSSClass("body")

	cf := gtk.NewLabel("Cloudflare is the cloud DNS manager: each host gets a name on your zone.")
	cf.SetWrap(true)
	cf.SetJustify(gtk.JustifyCenter)
	cf.SetXAlign(0.5)
	cf.SetMaxWidthChars(48)
	cf.AddCSSClass("dim-label")

	ln := gtk.NewLabel("Linode is the cloud VPS provider: a Debian instance is booted and Bedrud is installed on it.")
	ln.SetWrap(true)
	ln.SetJustify(gtk.JustifyCenter)
	ln.SetXAlign(0.5)
	ln.SetMaxWidthChars(48)
	ln.AddCSSClass("dim-label")

	logos := gtk.NewBox(gtk.OrientationHorizontal, 24)
	logos.SetHAlign(gtk.AlignCenter)
	logos.SetMarginTop(8)
	li := svgImage(linodeSVG, 48)
	ci := svgImage(cloudflareSVG, 48)
	logos.Append(li)
	logos.Append(ci)
	box.Append(lead)
	box.Append(logos)
	box.Append(cf)
	box.Append(ln)
	clamp.SetChild(box)
	p.SetChild(clamp)
	return p
}

func (a *App) initLinodePage() gtk.Widgetter {
	p := adw.NewStatusPage()
	if a.logoLinode == nil {
		a.logoLinode = svgPaintable(linodeSVG, 128)
	}
	if a.logoLinode != nil {
		p.SetPaintable(a.logoLinode)
	} else {
		p.SetIconName("network-server-symbolic")
	}
	p.SetTitle("Linode")
	p.SetDescription("VPS provider. Paste an API token from the Linode cloud manager.")

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(480)
	group := adw.NewPreferencesGroup()
	a.initLinode = adw.NewPasswordEntryRow()
	a.initLinode.SetTitle("Linode API token")
	group.Add(a.initLinode)
	clamp.SetChild(group)
	p.SetChild(clamp)
	return p
}

func (a *App) initCloudflarePage() gtk.Widgetter {
	p := adw.NewStatusPage()
	if a.logoCF == nil {
		a.logoCF = svgPaintable(cloudflareSVG, 128)
	}
	if a.logoCF != nil {
		p.SetPaintable(a.logoCF)
	} else {
		p.SetIconName("network-workgroup-symbolic")
	}
	p.SetTitle("Cloudflare DNS")
	p.SetDescription("DNS manager. Use an API token, or a legacy email + global API key. The zone is the domain new hosts are created under (example.com).")

	clamp := adw.NewClamp()
	clamp.SetMaximumSize(480)
	group := adw.NewPreferencesGroup()
	a.initCFTok = adw.NewPasswordEntryRow()
	a.initCFTok.SetTitle("API token (recommended)")
	group.Add(a.initCFTok)
	a.initCFEmail = adw.NewEntryRow()
	a.initCFEmail.SetTitle("Account email (legacy key)")
	group.Add(a.initCFEmail)
	a.initCFKey = adw.NewPasswordEntryRow()
	a.initCFKey.SetTitle("Global API key (legacy)")
	group.Add(a.initCFKey)
	a.initDomain = adw.NewEntryRow()
	a.initDomain.SetTitle("DNS zone (example.com)")
	group.Add(a.initDomain)
	a.initZone = adw.NewEntryRow()
	a.initZone.SetTitle("Zone ID (optional)")
	group.Add(a.initZone)
	clamp.SetChild(group)
	p.SetChild(clamp)
	return p
}

func (a *App) setInitPage(i int) {
	if a.initStack == nil {
		return
	}
	n := 3
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	a.initIndex = i
	a.initStack.SetVisibleChildName(strconv.Itoa(i))
	if a.initBack != nil {
		a.initBack.SetSensitive(i > 0)
	}
	if a.initNext != nil {
		if i+1 >= n {
			a.initNext.SetLabel("Initialize")
		} else {
			a.initNext.SetLabel("Continue")
		}
	}
	for j, d := range a.initDots {
		if j == i {
			d.RemoveCSSClass("dim-label")
		} else {
			d.AddCSSClass("dim-label")
		}
	}
}

func (a *App) initNextClicked() {
	n := 3
	if a.initIndex+1 < n {
		a.setInitPage(a.initIndex + 1)
		return
	}
	lt := a.initLinode.Text()
	tok := a.initCFTok.Text()
	em := a.initCFEmail.Text()
	key := a.initCFKey.Text()
	dom := a.initDomain.Text()
	zid := a.initZone.Text()
	if lt == "" {
		a.toastMsg("Linode API token is required")
		a.setInitPage(1)
		return
	}
	if dom == "" || (tok == "" && (em == "" || key == "")) {
		a.toastMsg("Need a DNS zone and a Cloudflare token or email+key")
		return
	}
	a.initNext.SetSensitive(false)
	a.beginOp("Initialize")
	go func() {
		_, err := hostcli.Init(a.bin, lt, tok, em, key, dom, zid)
		glib.IdleAdd(func() {
			a.initNext.SetSensitive(true)
			a.endOp(err)
			if err != nil {
				a.toastMsg(err.Error())
				return
			}
			a.toastMsg("Initialized")
			a.showPage("hosts")
			a.reloadHosts()
		})
	}()
}

func (a *App) buildHostsPage() gtk.Widgetter {
	outer := gtk.NewBox(gtk.OrientationVertical, 0)

	a.emptyPage = adw.NewStatusPage()
	a.emptyPage.SetIconName("network-server-symbolic")
	a.emptyPage.SetTitle("No hosts")
	a.emptyPage.SetDescription("Use + to create a host.")
	a.emptyPage.SetVExpand(true)

	a.scroller = gtk.NewScrolledWindow()
	a.scroller.SetVExpand(true)
	a.scroller.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)

	a.flow = gtk.NewFlowBox()
	a.flow.SetVAlign(gtk.AlignStart)
	a.flow.SetMaxChildrenPerLine(4)
	a.flow.SetMinChildrenPerLine(1)
	a.flow.SetSelectionMode(gtk.SelectionSingle)
	a.flow.SetActivateOnSingleClick(false)
	a.flow.ConnectChildActivated(func(child *gtk.FlowBoxChild) {
		id, err := strconv.Atoi(gtk.BaseWidget(child.Child()).Name())
		if err != nil {
			return
		}
		if a.pending != nil && a.pending.widget != nil && gtk.BaseWidget(child.Child()).Native() == gtk.BaseWidget(a.pending.widget).Native() {
			a.openCreateProgress()
			return
		}
		for _, h := range a.lastHosts {
			if h.ID == id {
				a.runText("view", h)
				return
			}
		}
	})
	a.flow.SetHomogeneous(false)
	a.flow.SetRowSpacing(12)
	a.flow.SetColumnSpacing(12)
	a.flow.AddCSSClass("hosts-flow")
	a.scroller.SetChild(a.flow)

	outer.Append(a.emptyPage)
	outer.Append(a.scroller)
	return outer
}

func (a *App) reloadHosts() {
	if a.bin == "" {
		return
	}
	if a.pending != nil {
		return
	}
	a.beginOp("Reload hosts")
	go func() {
		hosts, _, err := hostcli.List(a.bin)
		glib.IdleAdd(func() {
			a.endOp(err)
			if err != nil {
				a.toastMsg(err.Error())
				return
			}
			a.renderHosts(hosts)
		})
	}()
}

func (a *App) renderHostsKeep() {
	a.renderHosts(a.lastHosts)
}

func (a *App) renderHosts(hosts []hostcli.Host) {
	a.lastHosts = hosts
	for child := a.flow.FirstChild(); child != nil; {
		w := gtk.BaseWidget(child)
		next := w.NextSibling()
		a.flow.Remove(child)
		child = next
	}
	empty := len(hosts) == 0 && a.pending == nil
	a.emptyPage.SetVisible(empty)
	a.scroller.SetVisible(!empty)
	for i := range hosts {
		h := hosts[i]
		a.flow.Append(a.hostCard(h))
	}
	if a.pending != nil {
		a.flow.Insert(a.pending.widget, 0)
	}
}

func (a *App) hostCard(h hostcli.Host) gtk.Widgetter {
	card := gtk.NewBox(gtk.OrientationVertical, 8)
	card.AddCSSClass("card")
	card.AddCSSClass("host-card")
	card.SetMarginStart(4)
	card.SetMarginEnd(4)
	card.SetMarginTop(4)
	card.SetMarginBottom(4)

	title := gtk.NewLabel(h.FQDN)
	title.AddCSSClass("host-card-title")
	title.SetXAlign(0)
	title.SetWrap(true)
	title.SetSelectable(true)
	card.SetName(strconv.Itoa(h.ID))
	card.Append(title)

	meta := fmt.Sprintf("id %d  ·  %s", h.ID, h.IPv4)
	if h.Hourly != "" {
		meta += "  ·  " + h.Hourly
	}
	if h.Took != "" {
		meta += "\ncreate " + h.Took
	}
	if a.deletingID == h.ID {
		if a.deleteStage != "" {
			meta = a.deleteStage
		} else {
			meta = "Deleting…"
		}
	}
	sub := gtk.NewLabel(meta)
	sub.AddCSSClass("dim-label")
	sub.SetXAlign(0)
	sub.SetWrap(true)
	sub.SetSelectable(true)
	card.Append(sub)

	if a.deletingID == h.ID {
		return card
	}

	actions := gtk.NewBox(gtk.OrientationHorizontal, 6)
	actions.SetHomogeneous(true)

	view := gtk.NewButtonWithLabel("View")
	view.ConnectClicked(func() { a.runText("view", h) })
	actions.Append(view)

	admin := gtk.NewButtonWithLabel("Admin")
	admin.ConnectClicked(func() { a.runText("admin", h) })
	actions.Append(admin)

	open := gtk.NewButtonWithLabel("Open")
	open.ConnectClicked(func() {
		u := "https://" + h.FQDN
		gtk.ShowURI(&a.win.Window, u, gdk.CURRENT_TIME)
	})
	actions.Append(open)

	del := gtk.NewButtonWithLabel("Delete")
	del.AddCSSClass("destructive-action")
	del.ConnectClicked(func() { a.confirmDelete(h) })
	actions.Append(del)

	card.Append(actions)
	return card
}

func (a *App) runText(kind string, h hostcli.Host) {
	key := strconv.Itoa(h.ID)
	title := "View " + h.FQDN
	if kind == "admin" {
		title = "Admin " + h.FQDN
	}
	a.beginOp(title)
	go func() {
		var out string
		var err error
		if kind == "admin" {
			out, err = hostcli.Admin(a.bin, key)
		} else {
			out, err = hostcli.View(a.bin, key)
		}
		glib.IdleAdd(func() {
			a.endOp(err)
			if err != nil {
				a.toastMsg(err.Error())
				return
			}
			title := h.FQDN
			if kind == "admin" {
				title = "Admin · " + h.FQDN
			}
			a.showFieldsDialog(title, out)
		})
	}()
}

func prettyKey(k string) string {
	switch strings.ToLower(k) {
	case "id":
		return "ID"
	case "fqdn":
		return "FQDN"
	case "ipv4":
		return "IPv4"
	case "linode-id":
		return "Linode ID"
	case "linode-label":
		return "Linode label"
	case "admin-name":
		return "Admin name"
	case "admin-email":
		return "Admin email"
	case "admin-password":
		return "Admin password"
	case "create-time":
		return "Create time"
	case "created":
		return "Created"
	case "host":
		return "Host"
	case "zone":
		return "Zone"
	default:
		return strings.ReplaceAll(k, "-", " ")
	}
}

func (a *App) showLogDialog(title, body string) {
	d := adw.NewDialog()
	d.SetTitle(title)
	d.SetContentWidth(560)
	d.SetContentHeight(420)
	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(adw.NewHeaderBar())
	tv := gtk.NewTextView()
	tv.SetEditable(false)
	tv.SetMonospace(true)
	tv.SetWrapMode(gtk.WrapWordChar)
	tv.Buffer().SetText(body)
	sc := gtk.NewScrolledWindow()
	sc.SetVExpand(true)
	sc.SetChild(tv)
	toolbar.SetContent(sc)
	d.SetChild(toolbar)
	d.Present(a.win)
}

func (a *App) showFieldsDialog(title, body string) {
	fields := hostcli.ParseFields(body)
	d := adw.NewDialog()
	d.SetTitle(title)
	d.SetContentWidth(420)

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(adw.NewHeaderBar())

	if len(fields) == 0 {
		page := adw.NewStatusPage()
		page.SetTitle(title)
		page.SetDescription(strings.TrimSpace(body))
		toolbar.SetContent(page)
		d.SetChild(toolbar)
		d.Present(a.win)
		return
	}

	page := adw.NewPreferencesPage()
	group := adw.NewPreferencesGroup()
	for _, f := range fields {
		row := adw.NewActionRow()
		row.SetTitle(prettyKey(f.Key))
		row.SetSubtitle(f.Value)
		row.SetSubtitleSelectable(true)
		if f.Value != "" {
			val := f.Value
			copyBtn := gtk.NewButtonFromIconName("edit-copy-symbolic")
			copyBtn.SetVAlign(gtk.AlignCenter)
			copyBtn.AddCSSClass("flat")
			copyBtn.SetTooltipText("Copy")
			copyBtn.ConnectClicked(func() {
				a.win.Clipboard().SetText(val)
				a.toastMsg("Copied")
			})
			row.AddSuffix(copyBtn)
		}
		group.Add(row)
	}
	page.Add(group)
	toolbar.SetContent(page)
	d.SetChild(toolbar)
	d.Present(a.win)
}

func (a *App) confirmDelete(h hostcli.Host) {
	d := adw.NewAlertDialog("Delete host?", h.FQDN+" will be removed from Linode, DNS, and the local list.")
	d.AddResponse("cancel", "Cancel")
	d.AddResponse("delete", "Delete")
	d.SetResponseAppearance("delete", adw.ResponseDestructive)
	d.SetDefaultResponse("cancel")
	d.SetCloseResponse("cancel")
	d.ConnectResponse(func(response string) {
		if response != "delete" {
			return
		}
		a.beginOp("Delete " + h.FQDN)
		a.deletingID = h.ID
		a.deleteStage = "Deleting…"
		a.renderHostsKeep()
		id := strconv.Itoa(h.ID)
		go func() {
			var prog hostcli.CreateProgress
			_, err := hostcli.DeleteStream(a.bin, id, func(line string) {
				hostcli.ApplyDeleteLine(&prog, line)
				glib.IdleAdd(func() {
					if prog.Stage != "" {
						a.deleteStage = prog.Stage
						a.renderHostsKeep()
					}
				})
			})
			glib.IdleAdd(func() {
				a.deletingID = 0
				a.deleteStage = ""
				a.endOp(err)
				if err != nil {
					a.toastMsg(hostcli.ShortErr(err, ""))
					a.renderHostsKeep()
					return
				}
				a.toastMsg("Deleted " + h.FQDN)
				a.reloadHosts()
			})
		}()
	})
	d.Present(a.win)
}

func (a *App) onCreate() {
	d := adw.NewAlertDialog("Create a host?", "This boots a Linode, adds DNS, and installs Bedrud. It takes several minutes and starts billing.")
	d.AddResponse("cancel", "Cancel")
	d.AddResponse("create", "Create")
	d.SetResponseAppearance("create", adw.ResponseSuggested)
	d.SetDefaultResponse("cancel")
	d.SetCloseResponse("cancel")
	d.ConnectResponse(func(response string) {
		if response != "create" {
			return
		}
		a.runCreate()
	})
	d.Present(a.win)
}

func (a *App) runCreate() {
	if a.pending != nil {
		a.toastMsg("A host is already being created")
		return
	}
	a.beginOp("Create host")
	a.toastMsg("Creating host…")
	a.showPendingCard()
	go func() {
		out, err := hostcli.CreateStream(a.bin, func(line string) {
			glib.IdleAdd(func() { a.applyCreateLine(line) })
		})
		glib.IdleAdd(func() {
			a.finishPending(out, err)
		})
	}()
}

func (a *App) showPendingCard() {
	avg := 110 * time.Second
	if a.createAvg > 0 {
		avg = a.createAvg
	}
	cc := &createCard{start: time.Now(), avg: avg}
	cc.prog.Avg = avg
	cc.prog.Stage = "Starting"

	cc.spin = gtk.NewSpinner()
	cc.spin.Start()
	head := gtk.NewBox(gtk.OrientationHorizontal, 8)
	head.Append(cc.spin)
	creating := gtk.NewLabel("Creating")
	creating.AddCSSClass("heading")
	head.Append(creating)

	cc.title = gtk.NewLabel("New host…")
	cc.title.AddCSSClass("host-card-title")
	cc.title.SetXAlign(0)
	cc.title.SetWrap(false)
	cc.title.SetEllipsize(pango.EllipsizeEnd)
	cc.title.SetMaxWidthChars(28)

	cc.meta = gtk.NewLabel("Starting")
	cc.meta.AddCSSClass("dim-label")
	cc.meta.SetXAlign(0)
	cc.meta.SetWrap(false)
	cc.meta.SetEllipsize(pango.EllipsizeEnd)
	cc.meta.SetMaxWidthChars(28)

	cc.eta = gtk.NewLabel("0%  ·  ETA …")
	cc.eta.AddCSSClass("dim-label")
	cc.eta.SetXAlign(0)

	cc.bar = gtk.NewProgressBar()
	cc.bar.SetFraction(0)
	cc.bar.SetHExpand(true)

	cc.widget = gtk.NewBox(gtk.OrientationVertical, 8)
	cc.widget.AddCSSClass("card")
	cc.widget.AddCSSClass("host-card")
	cc.widget.AddCSSClass("host-card-creating")
	cc.widget.SetMarginStart(4)
	cc.widget.SetMarginEnd(4)
	cc.widget.SetMarginTop(4)
	cc.widget.SetMarginBottom(4)
	cc.widget.SetSizeRequest(300, -1)
	cc.widget.Append(head)
	cc.widget.Append(cc.title)
	cc.widget.Append(cc.meta)
	cc.widget.Append(cc.bar)
	cc.widget.Append(cc.eta)

	open := gtk.NewButtonWithLabel("Progress")
	open.AddCSSClass("suggested-action")
	open.ConnectClicked(func() { a.openCreateProgress() })
	cc.widget.Append(open)
	cc.widget.SetName("0")

	a.buildCreateProgressDialog(cc)
	a.pending = cc
	a.emptyPage.SetVisible(false)
	a.scroller.SetVisible(true)
	a.flow.Insert(cc.widget, 0)
	a.paintPending()
	a.openCreateProgress()
	cc.tick = glib.TimeoutAdd(200, func() bool {
		if a.pending != cc {
			return false
		}
		a.paintPending()
		return true
	})
}

func (a *App) buildCreateProgressDialog(cc *createCard) {
	d := adw.NewDialog()
	d.SetTitle("Creating host")
	d.SetContentWidth(520)
	d.SetContentHeight(480)
	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(adw.NewHeaderBar())

	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetMarginStart(18)
	box.SetMarginEnd(18)
	box.SetMarginTop(12)
	box.SetMarginBottom(18)

	cc.dlgTitle = gtk.NewLabel("New host…")
	cc.dlgTitle.AddCSSClass("title-2")
	cc.dlgTitle.SetXAlign(0)
	cc.dlgTitle.SetWrap(true)

	cc.dlgMeta = gtk.NewLabel("Starting")
	cc.dlgMeta.AddCSSClass("dim-label")
	cc.dlgMeta.SetXAlign(0)
	cc.dlgMeta.SetWrap(true)

	cc.dlgBar = gtk.NewProgressBar()
	cc.dlgBar.SetHExpand(true)

	cc.dlgEta = gtk.NewLabel("0%")
	cc.dlgEta.AddCSSClass("dim-label")
	cc.dlgEta.SetXAlign(0)

	tv := gtk.NewTextView()
	tv.SetEditable(false)
	tv.SetMonospace(true)
	tv.SetWrapMode(gtk.WrapChar)
	tv.AddCSSClass("card")
	cc.dlgBuf = tv.Buffer()
	sc := gtk.NewScrolledWindow()
	sc.SetVExpand(true)
	sc.SetMinContentHeight(220)
	sc.SetChild(tv)

	logLbl := gtk.NewLabel("Log")
	logLbl.AddCSSClass("heading")
	logLbl.SetXAlign(0)

	box.Append(cc.dlgTitle)
	box.Append(cc.dlgMeta)
	box.Append(cc.dlgBar)
	box.Append(cc.dlgEta)
	box.Append(logLbl)
	box.Append(sc)
	toolbar.SetContent(box)
	d.SetChild(toolbar)
	cc.dlg = d
}

func (a *App) openCreateProgress() {
	if a.pending == nil || a.pending.dlg == nil {
		return
	}
	a.pending.dlg.Present(a.win)
}

func (a *App) applyCreateLine(line string) {
	if a.pending == nil {
		return
	}
	a.pending.log.WriteString(line)
	a.pending.log.WriteByte('\n')
	if a.pending.dlgBuf != nil {
		a.pending.dlgBuf.SetText(a.pending.log.String())
	}
	hostcli.ApplyCreateLine(&a.pending.prog, line)
	if a.pending.prog.Avg > 0 {
		a.pending.avg = a.pending.prog.Avg
	}
	a.paintPending()
}

func (a *App) paintPending() {
	cc := a.pending
	if cc == nil {
		return
	}
	p := cc.prog
	if p.FQDN != "" {
		cc.title.SetText(p.FQDN)
		if cc.dlgTitle != nil {
			cc.dlgTitle.SetText(p.FQDN)
		}
		if cc.dlg != nil {
			cc.dlg.SetTitle(p.FQDN)
		}
	}
	if cc.failed {
		return
	}
	meta := p.Stage
	if p.IPv4 != "" {
		meta += "  ·  " + p.IPv4
	}
	if p.LinodeID != 0 {
		meta += fmt.Sprintf("  ·  linode %d", p.LinodeID)
	}
	if p.Hourly != "" {
		meta += "  ·  " + p.Hourly
	}
	cc.meta.SetText(meta)
	if cc.dlgMeta != nil {
		cc.dlgMeta.SetText(meta)
	}

	elapsed := time.Since(cc.start)
	avg := cc.avg
	if avg <= 0 {
		avg = 110 * time.Second
	}
	frac := float64(elapsed) / float64(avg)
	if frac < 0 {
		frac = 0
	}
	if !p.Ready && frac > 0.95 {
		frac = 0.95
	}
	if p.Ready {
		frac = 1
	}
	cc.bar.SetFraction(frac)
	if cc.dlgBar != nil {
		cc.dlgBar.SetFraction(frac)
	}

	pct := int(frac * 100)
	eta := fmt.Sprintf("%d%%  ·  ETA %s", pct, (avg - elapsed).Truncate(time.Second))
	if p.Ready {
		eta = fmt.Sprintf("100%%  ·  %s", elapsed.Truncate(time.Second))
	} else if elapsed >= avg {
		eta = fmt.Sprintf("%d%%  ·  Finishing…", pct)
	}
	cc.eta.SetText(eta)
	if cc.dlgEta != nil {
		cc.dlgEta.SetText(eta)
	}
	if p.FQDN != "" {
		a.opBtn.SetTooltipText("Create " + p.FQDN + " — " + p.Stage)
	}
	a.trayStatus("NeedsAttention")
}

func (a *App) finishPending(out string, err error) {
	if a.pending != nil && a.pending.tick != 0 {
		glib.SourceRemove(a.pending.tick)
		a.pending.tick = 0
	}
	a.endOp(err)
	if err != nil {
		if a.pending != nil {
			if out != "" {
				a.pending.log.Reset()
				a.pending.log.WriteString(out)
			}
			a.pending.failed = true
			a.pending.prog.Stage = "Failed"
			msg := hostcli.ShortErr(err, out)
			a.pending.meta.SetText(msg)
			a.pending.eta.SetText("Failed")
			a.pending.spin.Stop()
			a.pending.bar.SetFraction(1)
			if a.pending.dlgMeta != nil {
				a.pending.dlgMeta.SetText(msg)
			}
			if a.pending.dlgEta != nil {
				a.pending.dlgEta.SetText("Failed")
			}
			if a.pending.dlgBuf != nil && out != "" {
				a.pending.dlgBuf.SetText(out)
			}
			a.openCreateProgress()
		}
		a.toastMsg("Create failed — details in the progress window")
		return
	}
	a.pending = nil
	a.toastMsg("Host created")
	a.showFieldsDialog("Host created", out)
	a.reloadHosts()
}
