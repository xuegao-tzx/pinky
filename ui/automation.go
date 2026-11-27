package ui

import (
	"context"
	"strings"
	"time"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/cmd/pinky/locales"
	"github.com/DIYgod/pinky/internal/approutes"
	"github.com/DIYgod/pinky/internal/deskweb"
	"github.com/DIYgod/pinky/internal/skills"
)

// Automation is the skills library, app routes and embedded browser the
// window shows and the model drives.
type Automation struct {
	Skills *skills.Library
	Routes *approutes.Catalog
	Web    *deskweb.Session
}

// automationState is what the person is typing in Settings › Automation
// and in the browser page.
type automationState struct {
	skillPath                         string
	routeName, routeScheme            string
	routeAndroid, routeIOS, routePath string
	webURL                            string
	note                              string
	busy                              bool
}

func (a *app) settingsAutomation(c *mg.Context) {
	p := pal(c)
	settingsTitle(c, locales.SettingsAutomationTitle(), locales.SettingsAutomationDesc())
	au := a.opt.Automation
	if au == nil {
		mg.Text(c, locales.SettingsAutomationUnavailable()).FontSize(fsSmall).TextColor(p.ink2)
		return
	}
	st := &a.auto
	if st.note != "" {
		mg.Text(c, st.note).FontSize(fsSmall).TextColor(p.ink2)
	}
	mg.Column(c).Gap(10).Children(func() {
		settingsHeading(c, locales.SettingsSkillsHeading(), locales.SettingsSkillsDesc())
		group(c).Children(func() {
			settingRow(c, locales.SettingsSkillsPath(), "", true, func() {
				mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
					mg.TextInput(c.Key("skill-path"), &st.skillPath).Placeholder(locales.SettingsSkillsPath()).Width(220)
					if button(c, locales.SettingsSkillsImport(), btnOpt{small: true, kind: btnPrimary}).Clicked() {
						a.importSkill(st)
					}
					if button(c, locales.SettingsSkillsRefresh(), btnOpt{small: true}).Clicked() && au.Skills != nil {
						_ = au.Skills.Reload()
						st.note = ""
					}
				})
			})
		})
		if au.Skills == nil || len(au.Skills.List()) == 0 {
			mg.Text(c, locales.SettingsSkillsEmpty()).FontSize(fsSmall).TextColor(p.ink3)
		} else {
			for _, sk := range au.Skills.List() {
				sk := sk
				group(c).Children(func() {
					settingRow(c, sk.Name, sk.Description, true, func() {
						mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
							word := locales.SettingsSkillsOff()
							if sk.Enabled {
								word = locales.SettingsSkillsOn()
							}
							if button(c, word, btnOpt{small: true}).Clicked() {
								_ = au.Skills.SetEnabled(sk.ID, !sk.Enabled)
							}
							if button(c, locales.SettingsSkillsRemove(), btnOpt{small: true}).Clicked() {
								if err := au.Skills.Remove(sk.ID); err != nil {
									st.note = err.Error()
								}
							}
						})
					})
				})
			}
		}
	})
	mg.Column(c).Gap(10).Children(func() {
		settingsHeading(c, locales.SettingsRoutesHeading(), locales.SettingsRoutesDesc())
		group(c).Children(func() {
			settingRow(c, locales.SettingsRoutesName(), "", false, func() {
				mg.TextInput(c.Key("route-name"), &st.routeName).Width(180)
			})
			settingRow(c, locales.SettingsRoutesScheme(), "", false, func() {
				mg.TextInput(c.Key("route-scheme"), &st.routeScheme).Placeholder("hellobike://hellobike.com/").Width(260)
			})
			settingRow(c, locales.SettingsRoutesAndroid(), "", false, func() {
				mg.TextInput(c.Key("route-android"), &st.routeAndroid).Placeholder("com.example.app").Width(220)
			})
			settingRow(c, locales.SettingsRoutesIOS(), "", false, func() {
				mg.TextInput(c.Key("route-ios"), &st.routeIOS).Placeholder("com.example.app").Width(220)
			})
			settingRow(c, locales.SettingsRoutesPath(), "", true, func() {
				mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
					mg.TextInput(c.Key("route-path"), &st.routePath).Placeholder("{scheme}").Width(180)
					if button(c, locales.SettingsRoutesAdd(), btnOpt{small: true, kind: btnPrimary}).Clicked() {
						a.addRoute(st)
					}
				})
			})
		})
		if au.Routes == nil || len(au.Routes.List()) == 0 {
			mg.Text(c, locales.SettingsRoutesEmpty()).FontSize(fsSmall).TextColor(p.ink3)
		} else {
			for _, app := range au.Routes.List() {
				app := app
				line := app.Scheme
				if app.AndroidPackage != "" {
					line += "  " + app.AndroidPackage
				}
				group(c).Children(func() {
					settingRow(c, app.Name, line, true, func() {
						if button(c, locales.SettingsRoutesRemove(), btnOpt{small: true}).Clicked() {
							_ = au.Routes.Remove(app.Name)
						}
					})
				})
			}
		}
	})
	on := !a.settings.RemoteOpen
	if a.toggleRowFor(c, "remote-auth", locales.RemoteAuthToggle(), locales.RemoteAuthDesc(), true, &on) {
		a.settings.RemoteOpen = !on
		a.changed()
	}
	if button(c, locales.SidebarBrowser(), btnOpt{kind: btnPrimary}).Clicked() {
		a.page = pageBrowser
	}
}

func (a *app) importSkill(st *automationState) {
	au := a.opt.Automation
	if au == nil || au.Skills == nil {
		return
	}
	if err := au.Skills.Import(strings.TrimSpace(st.skillPath)); err != nil {
		st.note = err.Error()
		return
	}
	st.note = ""
	st.skillPath = ""
}

func (a *app) addRoute(st *automationState) {
	au := a.opt.Automation
	if au == nil || au.Routes == nil {
		return
	}
	path := strings.TrimSpace(st.routePath)
	var routes []approutes.Route
	if path != "" {
		routes = []approutes.Route{{Name: "open", Path: path}}
	}
	err := au.Routes.Add(approutes.App{
		Name: st.routeName, Scheme: st.routeScheme,
		AndroidPackage: st.routeAndroid, IOSBundleID: st.routeIOS, Routes: routes,
	})
	if err != nil {
		st.note = err.Error()
		return
	}
	st.note = ""
	st.routeName, st.routeScheme, st.routeAndroid, st.routeIOS, st.routePath = "", "", "", "", ""
}

func (a *app) browserPage(c *mg.Context) {
	p := pal(c)
	pageHeader(c, func() { pageTitle(c, locales.BrowserTitle()) })
	au := a.opt.Automation
	mg.Scroll(c).Grow(1).Padding(28, 40).Children(func() {
		mg.Column(c).MaxWidth(720).Gap(16).Children(func() {
			mg.Text(c, locales.BrowserDesc()).FontSize(fsSmall).LineHeight(1.45).TextColor(p.ink2)
			if au == nil || au.Web == nil {
				mg.Text(c, locales.SettingsAutomationUnavailable()).FontSize(fsSmall).TextColor(p.ink2)
				return
			}
			web := au.Web
			st := &a.auto
			mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
				in := mg.TextInput(c.Key("web-url"), &st.webURL).Label(locales.BrowserURL()).Placeholder("https://").Grow(1)
				goNow := button(c, locales.BrowserGo(), btnOpt{kind: btnPrimary}).Clicked() || in.Submitted()
				if goNow {
					a.browse(st.webURL, web.Navigate)
				}
				if button(c, locales.BrowserBack(), btnOpt{}).Clicked() {
					a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return web.Back(ctx) })
				}
				if button(c, locales.BrowserForward(), btnOpt{}).Clicked() {
					a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return web.Forward(ctx) })
				}
				if button(c, locales.BrowserReload(), btnOpt{}).Clicked() {
					a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return web.Reload(ctx) })
				}
				if button(c, locales.BrowserSave(), btnOpt{}).Clicked() {
					if err := web.SaveBookmark("", st.webURL); err != nil {
						st.note = err.Error()
					}
				}
			})
			if button(c, locales.BrowserWindow(), btnOpt{}).Clicked() && !st.busy {
				st.busy = true
				go func() {
					ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
					defer cancel()
					err := web.UseChrome(ctx)
					a.post(func() {
						st.busy = false
						if err != nil {
							st.note = err.Error()
						} else {
							st.note = locales.BrowserWindowOpen()
						}
					})
				}()
			}
			if st.note != "" {
				mg.Text(c, st.note).FontSize(fsSmall).TextColor(p.ink2)
			}
			url, title := web.State()
			if url != "" {
				mg.Text(c, title).FontSize(fsBody).FontWeight(fwSemibold)
				mg.Text(c, url).Font("monospace").FontSize(fsSmall).TextColor(p.ink2).Selectable()
			} else {
				mg.Text(c, locales.BrowserEmpty()).FontSize(fsSmall).TextColor(p.ink3)
			}
			page, _ := web.Content(context.Background())
			if page.Text != "" {
				mg.Text(c, page.Text).FontSize(fsSmall).LineHeight(1.4)
			}
			if len(web.Bookmarks()) > 0 {
				mg.Text(c, locales.BrowserSaved()).FontSize(fsSmall).FontWeight(fwSemibold).TextColor(p.ink2)
				for _, b := range web.Bookmarks() {
					b := b
					if button(c, b.Title, btnOpt{small: true}).Clicked() {
						st.webURL = b.URL
						a.browse(b.URL, web.Navigate)
					}
				}
			}
		})
	})
}

func (a *app) browse(url string, fn func(context.Context, string) (deskweb.Page, error)) {
	st := &a.auto
	if st.busy {
		return
	}
	st.busy = true
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()
		page, err := fn(ctx, url)
		a.post(func() {
			st.busy = false
			if err != nil {
				st.note = err.Error()
				return
			}
			st.note = ""
			if page.URL != "" {
				st.webURL = page.URL
			}
		})
	}()
}
