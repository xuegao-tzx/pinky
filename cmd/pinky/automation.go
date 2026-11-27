package main

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/DIYgod/pinky/internal/approutes"
	"github.com/DIYgod/pinky/internal/deskweb"
	"github.com/DIYgod/pinky/internal/mcp"
	"github.com/DIYgod/pinky/internal/pilot"
	"github.com/DIYgod/pinky/internal/secret"
	"github.com/DIYgod/pinky/internal/skills"
	"github.com/DIYgod/pinky/ui"
)

// automation is the skills library, app routes and embedded browser for
// one process.
type automation struct {
	Skills *skills.Library
	Routes *approutes.Catalog
	Web    *deskweb.Session
}

func (a *automation) mcp() mcp.Automation {
	if a == nil {
		return mcp.Automation{}
	}
	return mcp.Automation{Skills: a.Skills, Routes: a.Routes, Web: a.Web}
}

func (a *automation) Close() {
	if a != nil && a.Web != nil {
		a.Web.Close()
	}
}

func (a *automation) ui() *ui.Automation {
	if a == nil {
		return nil
	}
	return &ui.Automation{Skills: a.Skills, Routes: a.Routes, Web: a.Web}
}

// openAutomation loads the folders under the gateway's data directory.
func openAutomation(rg *runningGateway, log *slog.Logger) *automation {
	data := rg.DataDir
	lib, err := skills.Open(filepath.Join(data, "skills.json"))
	if err != nil {
		log.Warn("skills", "err", err)
		lib, _ = skills.Open("")
	}
	routes, err := approutes.Open(filepath.Join(rg.Dirs.Config, "app-routes.yaml"))
	if err != nil {
		log.Warn("app routes", "err", err)
		routes, _ = approutes.Open("")
	}
	web, err := deskweb.Open(filepath.Join(data, "webview"))
	if err != nil {
		log.Warn("browser", "err", err)
		web, _ = deskweb.Open("")
	}
	a := &automation{Skills: lib, Routes: routes, Web: web}
	rg.MCP.SetAutomation(a.mcp())
	return a
}

// startDaemonPilot lets remote agents hand tasks to the configured model
// while the window is closed. Approvals are off, so the window is not
// required to answer them.
func startDaemonPilot(rg *runningGateway, auto *automation, s ui.Settings) *pilot.Runner {
	ask := pilot.New(pilot.Options{
		Server:  rg.MCP,
		Version: version,
		Key: func(ctx context.Context, name string) (string, error) {
			return secret.Get(ctx, secret.System(), secret.ModelKeyAccount(name))
		},
		Describe: ui.DescribePilot,
		SkillPrompt: func() string {
			if auto == nil || auto.Skills == nil {
				return ""
			}
			return auto.Skills.Prompt()
		},
	})
	cfg := s
	rg.MCP.SetTasks(ask.ForAgents(func() (pilot.Config, bool) {
		return cfg.PilotConfig(), cfg.AskPinky
	}))
	return ask
}
