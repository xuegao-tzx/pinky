package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DIYgod/pinky/internal/driver"
	"github.com/DIYgod/pinky/internal/gateway"
)

// extraToolNames are the tools beyond the phone_* set.
var extraToolNames = []string{
	"open_app_route",
	"run_agent_task", "agent_task_status", "agent_task_wait", "agent_task_cancel",
	"skill_list", "skill_read", "skill_run",
	"webview_navigate", "webview_back", "webview_forward", "webview_reload", "webview_state",
	"webview_content", "webview_click", "webview_type", "webview_scroll", "webview_eval", "webview_screenshot",
}

func (s *Server) addExtraTools(srv *sdk.Server, remote bool) {
	read := &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolp(false)}
	act := &sdk.ToolAnnotations{DestructiveHint: boolp(false), OpenWorldHint: boolp(true)}

	sdk.AddTool(srv, &sdk.Tool{Name: "open_app_route", Title: "Open an app route",
		Description: "Open a deep link on the phone. Use a preset from the app route config (name, Android package or iOS bundle id) and a route shortcut, or pass uri. " +
			"Android runs am start -a android.intent.action.VIEW -d <uri>, with -p <package> when the preset has one. iOS opens the URL through Pinky's iOS channel, in the bundle when the preset has one. " +
			"It does not ask for approval. params are name=value pairs that fill {name} in the route.",
		InputSchema: schema[routeIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in routeIn) (*sdk.CallToolResult, any, error) {
			return s.openRoute(ctx, in), nil, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "run_agent_task", Title: "Run a task on Pinky's model",
		Description: "Hand a natural-language task to the model configured in Pinky. It uses the phone, imported skills and the embedded browser, then you follow it with agent_task_wait. " +
			"skills names skill ids to follow; omit it to use the enabled skills. The result is structured: status, task_id, and later the summary, steps and screenshot paths.",
		InputSchema: schema[agentTaskIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in agentTaskIn) (*sdk.CallToolResult, any, error) {
			return s.runAgentTask(req, remote, in), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "agent_task_status", Title: "Agent task status",
		Description: "The structured status of a task from run_agent_task: status, summary, steps and screenshot paths.",
		InputSchema: schema[agentIDIn](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in agentIDIn) (*sdk.CallToolResult, any, error) {
			t, res := s.taskFind(req, remote, in.TaskID)
			if res != nil {
				return res, nil, nil
			}
			return textResult(s.structuredTask(t), false), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "agent_task_wait", Title: "Wait for an agent task",
		Description: "Wait until the task from run_agent_task ends, or until timeout_ms (default 30000, at most 45000). Returns the structured result.",
		InputSchema: schema[agentWaitIn](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in agentWaitIn) (*sdk.CallToolResult, any, error) {
			return s.waitAgentTask(ctx, req, remote, in), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "agent_task_cancel", Title: "Cancel an agent task",
		Description: "Stop a task from run_agent_task.",
		InputSchema: schema[agentIDIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in agentIDIn) (*sdk.CallToolResult, any, error) {
			return s.cancelAgentTask(req, remote, in.TaskID), nil, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "skill_list", Title: "List skills",
		Description: "List imported skills: id, name, description, folder and whether each is enabled.",
		InputSchema: schema[struct{}](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return textResult(s.skillList(), false), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "skill_read", Title: "Read a skill",
		Description: "Read the full SKILL.md of an imported skill and the names of the files beside it.",
		InputSchema: schema[skillIDIn](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in skillIDIn) (*sdk.CallToolResult, any, error) {
			return s.skillRead(in.ID), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "skill_run", Title: "Run a skill command",
		Description: "Run a command in an imported skill's folder, with the system shell, so bash, python, node and adb work. The command is the skill's. Output is capped.",
		InputSchema: schema[skillRunIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in skillRunIn) (*sdk.CallToolResult, any, error) {
			return s.skillRun(ctx, in), nil, nil
		})

	sdk.AddTool(srv, &sdk.Tool{Name: "webview_navigate", Title: "Open a page in the embedded browser",
		Description: "Open url in Pinky's embedded browser. Returns the url, title, text and elements.",
		InputSchema: schema[webURLIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in webURLIn) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Navigate(ctx, in.URL)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_back", Title: "Embedded browser back",
		Description: "Go back in the embedded browser.",
		InputSchema: schema[struct{}](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Back(ctx)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_forward", Title: "Embedded browser forward",
		Description: "Go forward in the embedded browser.",
		InputSchema: schema[struct{}](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Forward(ctx)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_reload", Title: "Reload the embedded browser",
		Description: "Reload the embedded browser's current page.",
		InputSchema: schema[struct{}](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Reload(ctx)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_state", Title: "Embedded browser address",
		Description: "The embedded browser's current URL and title.",
		InputSchema: schema[struct{}](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				u, title := s.web.State()
				return fmt.Sprintf("url: %s\ntitle: %s\nmode: %s", u, title, s.web.Mode()), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_content", Title: "Read the embedded browser",
		Description: "The embedded page: URL, title, visible text and the interactive elements (selector and text).",
		InputSchema: schema[struct{}](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Content(ctx)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_click", Title: "Click in the embedded browser",
		Description: "Click an element by CSS selector or by text.",
		InputSchema: schema[webClickIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in webClickIn) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Click(ctx, in.Selector, in.Text)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_type", Title: "Type in the embedded browser",
		Description: "Type into an element by CSS selector or by its text or placeholder.",
		InputSchema: schema[webTypeIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in webTypeIn) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Type(ctx, in.Selector, in.Text, in.Value)
				if err != nil {
					return "", err
				}
				return p.Summary(), nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_scroll", Title: "Scroll the embedded browser",
		Description: "Scroll the embedded page by dy pixels (negative scrolls up).",
		InputSchema: schema[webScrollIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in webScrollIn) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				if err := s.web.Scroll(ctx, in.DY); err != nil {
					return "", err
				}
				return "scrolled", nil
			}), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_eval", Title: "Run JavaScript in the embedded browser",
		Description: "Evaluate JavaScript in the embedded page and return the value. Without Chrome, document.title, document.URL, document.body.innerText and document.querySelector(sel).textContent or .value work.",
		InputSchema: schema[webEvalIn](nil), Annotations: act},
		func(ctx context.Context, req *sdk.CallToolRequest, in webEvalIn) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) { return s.web.Eval(ctx, in.Expression) }), nil, nil
		})
	sdk.AddTool(srv, &sdk.Tool{Name: "webview_screenshot", Title: "Screenshot the embedded browser",
		Description: "Save a screenshot of the embedded page and return its path. With Chrome it is a PNG; otherwise a text capture of the page.",
		InputSchema: schema[struct{}](nil), Annotations: read},
		func(ctx context.Context, req *sdk.CallToolRequest, in struct{}) (*sdk.CallToolResult, any, error) {
			return s.webDo(func() (string, error) {
				p, err := s.web.Screenshot(ctx)
				if err != nil {
					return "", err
				}
				return "screenshot: " + p, nil
			}), nil, nil
		})
}

type routeIn struct {
	Device string   `json:"device,omitempty" jsonschema:"The phone, by id or name. Optional when one phone is connected."`
	App    string   `json:"app,omitempty" jsonschema:"Preset name, Android package or iOS bundle id."`
	Route  string   `json:"route,omitempty" jsonschema:"Route shortcut name, or a path appended to the scheme."`
	URI    string   `json:"uri,omitempty" jsonschema:"A full URI. When set, route is ignored."`
	Params []string `json:"params,omitempty" jsonschema:"Parameter pairs written as name and value with an equals sign between them, such as bikeId=42, which fill braces of that name in the route."`
}

type agentTaskIn struct {
	Task   string   `json:"task" jsonschema:"The task in words: what to do and what to report back."`
	Device string   `json:"device,omitempty" jsonschema:"The phone, by id or name. Optional when one phone is connected."`
	Skills []string `json:"skills,omitempty" jsonschema:"Skill ids to follow. Omit to use every enabled skill."`
}

type agentIDIn struct {
	TaskID string `json:"task_id" jsonschema:"The task_id run_agent_task returned."`
}

type agentWaitIn struct {
	TaskID    string `json:"task_id" jsonschema:"The task_id run_agent_task returned."`
	TimeoutMS int    `json:"timeout_ms,omitempty" jsonschema:"Wait at most this many milliseconds, 1000 to 45000; default 30000."`
}

type skillIDIn struct {
	ID string `json:"id" jsonschema:"The skill id from skill_list."`
}

type skillRunIn struct {
	ID      string `json:"id" jsonschema:"The skill id."`
	Command string `json:"command" jsonschema:"The command to run in the skill folder, such as bash scripts/setup.sh or python make_data.py."`
}

type webURLIn struct {
	URL string `json:"url" jsonschema:"The address to open."`
}

type webClickIn struct {
	Selector string `json:"selector,omitempty" jsonschema:"A CSS selector, such as #lock."`
	Text     string `json:"text,omitempty" jsonschema:"Text of the element, when there is no selector."`
}

type webTypeIn struct {
	Selector string `json:"selector,omitempty" jsonschema:"A CSS selector."`
	Text     string `json:"text,omitempty" jsonschema:"Placeholder or label of the field."`
	Value    string `json:"value" jsonschema:"The text to type."`
}

type webScrollIn struct {
	DY int `json:"dy" jsonschema:"Pixels to scroll. Negative scrolls up."`
}

type webEvalIn struct {
	Expression string `json:"expression" jsonschema:"JavaScript to evaluate."`
}

func textResult(s string, isErr bool) *sdk.CallToolResult {
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: s}}, IsError: isErr}
}

func (s *Server) openRoute(ctx context.Context, in routeIn) *sdk.CallToolResult {
	s.mu.Lock()
	cat := s.routes
	s.mu.Unlock()
	if cat == nil {
		return textResult("error: unsupported\nNo app routes are configured.", true)
	}
	params := map[string]string{}
	for _, p := range in.Params {
		k, v, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return textResult("error: invalid_argument\nparams are name=value.", true)
		}
		params[strings.TrimSpace(k)] = v
	}
	target, err := cat.Resolve(in.App, in.Route, in.URI, params)
	if err != nil {
		return textResult("error: invalid_argument\n"+err.Error(), true)
	}
	device := strings.TrimSpace(in.Device)
	if device == "" {
		id, res := s.taskDevice("")
		if res != nil {
			return res
		}
		device = id
	} else if id, res := s.taskDevice(device); res == nil {
		device = id
	}
	id := target.AndroidPackage
	if s.platform(device) == driver.IOS {
		id = target.IOSBundleID
	}
	if err := s.g.OpenURI(ctx, device, target.URI, id); err != nil {
		return textResult("error: internal\n"+err.Error(), true)
	}
	return textResult(fmt.Sprintf("settled\nopened %s\nuri: %s\napp: %s\nandroid_package: %s\nios_bundle_id: %s",
		target.URI, target.URI, target.App, target.AndroidPackage, target.IOSBundleID), false)
}

func (s *Server) platform(id string) driver.Platform {
	if s.g == nil {
		return ""
	}
	for _, d := range s.g.Devices() {
		if d.Info.ID == id {
			return d.Info.Platform
		}
	}
	return ""
}

func (s *Server) runAgentTask(req *sdk.CallToolRequest, remote bool, in agentTaskIn) *sdk.CallToolResult {
	tr, agent, res := s.taskAgent(req, remote)
	if res != nil {
		return res
	}
	if strings.TrimSpace(in.Task) == "" {
		return textResult("error: invalid_argument\ntask is required.", true)
	}
	device, res := s.taskDevice(strings.TrimSpace(in.Device))
	if res != nil {
		return res
	}
	s.mu.Lock()
	lib := s.skills
	s.mu.Unlock()
	var brief string
	if lib != nil {
		brief = lib.Brief(in.Skills)
	} else if len(in.Skills) > 0 {
		return textResult("error: unsupported\nNo skills are imported.", true)
	}
	t, err := tr.Run(TaskRequest{Agent: agent, Device: device, Task: in.Task, SkillText: brief})
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			return render(&gateway.Result{Error: te.Code, Message: te.Message})
		}
		return textResult("error: internal\n"+err.Error(), true)
	}
	return textResult(s.structuredTask(t), false)
}

func (s *Server) waitAgentTask(ctx context.Context, req *sdk.CallToolRequest, remote bool, in agentWaitIn) *sdk.CallToolResult {
	t, res := s.taskFind(req, remote, in.TaskID)
	if res != nil {
		return res
	}
	tr, agent, res := s.taskAgent(req, remote)
	if res != nil {
		return res
	}
	timeout := taskWaitDefault
	if in.TimeoutMS > 0 {
		timeout = min(max(time.Duration(in.TimeoutMS)*time.Millisecond, time.Second), taskWaitMax)
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for t.Running() && wctx.Err() == nil {
		next, err := tr.Wait(wctx, agent.Name, t.ID, t.Version)
		if err != nil {
			break
		}
		t = next
	}
	return textResult(s.structuredTask(t), false)
}

func (s *Server) cancelAgentTask(req *sdk.CallToolRequest, remote bool, id string) *sdk.CallToolResult {
	t, res := s.taskFind(req, remote, id)
	if res != nil {
		return res
	}
	tr, agent, res := s.taskAgent(req, remote)
	if res != nil {
		return res
	}
	t, err := tr.Cancel(agent.Name, t.ID)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			return render(&gateway.Result{Error: te.Code, Message: te.Message})
		}
		return textResult("error: internal\n"+err.Error(), true)
	}
	return textResult(s.structuredTask(t), false)
}

func (s *Server) structuredTask(t Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "status: %s\ntask_id: %s\ndevice: %s\nsteps: %d\nsummary: %s\n",
		taskStatus(t), t.ID, t.Device, t.Steps, strings.TrimSpace(t.Summary))
	if len(t.Log) > 0 {
		b.WriteString("steps_detail:\n")
		for i, st := range t.Log {
			fmt.Fprintf(&b, "%d. %s %s", t.First+i, st.Tool, st.Status)
			if st.Intent != "" {
				fmt.Fprintf(&b, " — %s", st.Intent)
			}
			b.WriteByte('\n')
			if st.Output != "" {
				for _, line := range strings.Split(st.Output, "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					fmt.Fprintf(&b, "   %s\n", line)
				}
			}
		}
	}
	var shots []string
	for _, st := range t.Log {
		for _, line := range strings.Split(st.Output, "\n") {
			line = strings.TrimSpace(line)
			if path, ok := strings.CutPrefix(line, "screenshot: "); ok && path != "" {
				shots = append(shots, path)
			}
			if path, ok := strings.CutPrefix(line, "saved: "); ok {
				path = strings.TrimSpace(strings.Split(path, "·")[0])
				if path != "" {
					shots = append(shots, path)
				}
			}
		}
	}
	s.mu.Lock()
	web := s.web
	s.mu.Unlock()
	if web != nil {
		if p := web.LastScreenshot(); p != "" {
			shots = append(shots, p)
		}
	}
	if len(shots) > 0 {
		b.WriteString("screenshots:\n")
		seen := map[string]bool{}
		for _, p := range shots {
			if seen[p] {
				continue
			}
			seen[p] = true
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Server) skillList() string {
	s.mu.Lock()
	lib := s.skills
	s.mu.Unlock()
	if lib == nil {
		return "No skills imported."
	}
	list := lib.List()
	if len(list) == 0 {
		return "No skills imported."
	}
	var b strings.Builder
	for _, sk := range list {
		on := "disabled"
		if sk.Enabled {
			on = "enabled"
		}
		fmt.Fprintf(&b, "%s · %s · %s\n  %s\n  %s\n", sk.ID, sk.Name, on, oneLine(sk.Description, 200), sk.Dir)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *Server) skillRead(id string) *sdk.CallToolResult {
	s.mu.Lock()
	lib := s.skills
	s.mu.Unlock()
	if lib == nil {
		return textResult("error: unsupported\nNo skills imported.", true)
	}
	body, files, err := lib.Read(id)
	if err != nil {
		return textResult("error: not_found\n"+err.Error(), true)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\nfiles: %s\n\n%s", strings.TrimSpace(id), strings.Join(files, ", "), body)
	return textResult(b.String(), false)
}

func (s *Server) skillRun(ctx context.Context, in skillRunIn) *sdk.CallToolResult {
	s.mu.Lock()
	lib := s.skills
	s.mu.Unlock()
	if lib == nil {
		return textResult("error: unsupported\nNo skills imported.", true)
	}
	res, err := lib.Run(ctx, in.ID, in.Command)
	var b strings.Builder
	fmt.Fprintf(&b, "dir: %s\nexit: %d\n", res.Dir, res.Exit)
	if res.Stdout != "" {
		fmt.Fprintf(&b, "stdout:\n%s\n", res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprintf(&b, "stderr:\n%s\n", res.Stderr)
	}
	if err != nil && res.Exit == 0 && !res.TimedOut {
		return textResult("error: internal\n"+err.Error()+"\n"+b.String(), true)
	}
	if res.TimedOut {
		return textResult("error: internal\n"+b.String()+err.Error(), true)
	}
	return textResult(strings.TrimRight(b.String(), "\n"), res.Exit != 0)
}

func (s *Server) webDo(fn func() (string, error)) *sdk.CallToolResult {
	s.mu.Lock()
	web := s.web
	s.mu.Unlock()
	if web == nil {
		return textResult("error: unsupported\nThe embedded browser is not open.", true)
	}
	text, err := fn()
	if err != nil {
		return textResult("error: internal\n"+err.Error(), true)
	}
	return textResult(text, false)
}
