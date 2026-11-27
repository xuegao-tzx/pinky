package ui

import (
	"context"
	"fmt"
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

func (a *app) ensureBrowser() {
	if a.browser != nil || a.browserErr != nil || a.main == nil {
		return
	}
	h, err := newBrowserHost(a.main)
	if err != nil {
		a.browserErr = err
		if a.opt.Automation != nil && a.opt.Automation.Web != nil {
			a.opt.Automation.Web.FailEngine(err)
		}
		return
	}
	h.poke = func() {
		if a.main != nil {
			a.main.Invalidate()
		}
	}
	h.show = a.presentBrowser
	a.browser = h
	if a.opt.Automation != nil && a.opt.Automation.Web != nil {
		a.opt.Automation.Web.UseEngine(h)
	}
}

// presentBrowser shows the Browser page so its web view is given a size.
func (a *app) presentBrowser(ctx context.Context) error {
	done := make(chan struct{})
	a.post(func() {
		a.page = pageBrowser
		close(done)
	})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// standbyBrowser keeps the active web view at a real size while another
// page is showing. It is fully transparent, so the page on top is what
// the person sees, and a snapshot still has a viewport.
func (a *app) standbyBrowser(c *mg.Context) {
	if a.browser == nil || a.page == pageBrowser {
		return
	}
	view, _ := a.browser.views()
	if view == nil {
		return
	}
	mg.WebView(c, view).Absolute().Left(0).Top(0).Width(browserViewportW).Height(browserViewportH).Opacity(0).PassThrough()
}

func (a *app) browserPage(c *mg.Context) {
	p := pal(c)
	pageHeader(c, func() { pageTitle(c, locales.BrowserTitle()) })
	a.ensureBrowser()
	host := a.browser
	st := &a.auto
	mg.Column(c).Grow(1).MinHeight(0).Padding(12, 16).Gap(8).Children(func() {
		mg.Text(c, locales.BrowserEmbedded()).FontSize(fsSmall).TextColor(p.ink2)
		if host == nil {
			if a.browserErr != nil {
				mg.Text(c, a.browserErr.Error()).FontSize(fsSmall).TextColor(p.red)
			} else {
				mg.Text(c, locales.BrowserNoWindow()).FontSize(fsSmall).TextColor(p.ink2)
			}
			return
		}
		view, tabs := host.views()
		mg.Row(c).Gap(6).AlignItems(mg.Center).Children(func() {
			for _, tb := range tabs {
				tb := tb
				label := tb.Title
				if label == "" {
					label = tb.URL
				}
				if label == "" {
					label = tb.ID
				}
				opt := btnOpt{small: true}
				if tb.Active {
					opt.kind = btnPrimary
				}
				if button(c.Key("tab-"+tb.ID), label, opt).Clicked() {
					_ = host.SwitchTab(context.Background(), tb.ID)
					st.webURL = tb.URL
				}
			}
			if button(c, locales.BrowserNewTab(), btnOpt{small: true}).Clicked() {
				if tab, err := host.NewTab(context.Background(), ""); err != nil {
					st.note = err.Error()
				} else {
					st.webURL = tab.URL
				}
			}
			if button(c, locales.BrowserCloseTab(), btnOpt{small: true}).Clicked() {
				if err := host.CloseTab(context.Background(), ""); err != nil {
					st.note = err.Error()
				}
			}
		})
		mg.Row(c).Gap(8).AlignItems(mg.Center).Children(func() {
			in := mg.TextInput(c.Key("web-url"), &st.webURL).Label(locales.BrowserURL()).Placeholder("https://").Grow(1)
			if button(c, locales.BrowserGo(), btnOpt{kind: btnPrimary}).Clicked() || in.Submitted() {
				a.browse(st.webURL, func(ctx context.Context, url string) (deskweb.Page, error) {
					return host.Navigate(ctx, "", url)
				})
			}
			if button(c, locales.BrowserBack(), btnOpt{}).Clicked() {
				a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return host.Back(ctx, "") })
			}
			if button(c, locales.BrowserForward(), btnOpt{}).Clicked() {
				a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return host.Forward(ctx, "") })
			}
			if button(c, locales.BrowserReload(), btnOpt{}).Clicked() {
				a.browse("", func(ctx context.Context, _ string) (deskweb.Page, error) { return host.Reload(ctx, "") })
			}
			if au := a.opt.Automation; au != nil && au.Web != nil && button(c, locales.BrowserSave(), btnOpt{}).Clicked() {
				if err := au.Web.SaveBookmark("", st.webURL); err != nil {
					st.note = err.Error()
				}
			}
		})
		if au := a.opt.Automation; au != nil && au.Web != nil {
			if marks := au.Web.Bookmarks(); len(marks) > 0 {
				mg.Row(c).Gap(6).AlignItems(mg.Center).Children(func() {
					mg.Text(c, locales.BrowserSaved()).FontSize(fsSmall).TextColor(p.ink2)
					for i, b := range marks {
						b := b
						label := b.Title
						if label == "" {
							label = b.URL
						}
						if button(c.Key(fmt.Sprintf("mark-%d", i)), label, btnOpt{small: true}).Clicked() {
							st.webURL = b.URL
							a.browse(b.URL, func(ctx context.Context, url string) (deskweb.Page, error) {
								return host.Navigate(ctx, "", url)
							})
						}
					}
				})
			}
		}
		if st.note != "" {
			mg.Text(c, st.note).FontSize(fsSmall).TextColor(p.ink2)
		}
		if view != nil {
			mg.WebView(c, view).Grow(1).MinHeight(240).Radius(8)
		}
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
