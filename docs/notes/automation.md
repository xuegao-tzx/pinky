# Automation: open actions, skills, app routes, remote tasks, embedded browser

This note is the build that removes Pinky's approval gates and adds skills, app deep links, a remote task tool and an embedded browser. The phone drivers, the gateway session and the existing `phone_*` tools are unchanged aside from the policy switch.

## What runs without asking

`gateway.Options.Unrestricted` is set when the app or `pinky daemon` starts. It turns on `policy.Engine.SetUnrestricted` and skips the new-agent question.

While that is on:

- `Evaluate` allows every action, including payments, installs, launches and typing.
- `Blocked` hides no app, so banking, wallet and password-manager screens are visible.
- `CredentialTypingAllowed` is true on a real phone and on a virtual phone, so a password field and a hidden passcode keypad accept input. The task is still expected to supply the secret; the model prompt says not to invent one, and not to type the phone's own passcode unless the task includes it.
- Read-only mode and agent or task app lists do not refuse the action.

The rules file is left on disk. Unit tests that build an engine without `SetUnrestricted` still cover the old gates.

## Skills

`internal/skills` reads a folder. The folder is one skill when it contains `SKILL.md`, and it is also scanned one level down for more. Front matter is a `---` block with `name` and `description`.

The library is `skills.json` in Pinky's data folder: the imported roots and which ids are disabled. Settings › Automation imports, refreshes, enables, disables and removes them. Ask Pinky adds `Library.Prompt` to the first message of a task that did not name skills of its own.

Tools, on the MCP server and therefore on Ask Pinky's model:

| Tool | What it does |
| --- | --- |
| `skill_list` | Id, name, enabled, description, folder |
| `skill_read` | The full `SKILL.md` and the names of the files beside it |
| `skill_run` | A command in that folder, through `sh -c` (or `cmd /c` on Windows), for up to 2 minutes |

## App routes

`app-routes.yaml` in the config folder, and Settings › Automation, hold presets. A preset has `name`, `scheme`, `android_package`, `ios_bundle_id` and `routes`. A route `path` may use `{scheme}` and `{param}` filled from `params` (`bikeId=42`).

`open_app_route` resolves the preset and calls `gateway.OpenURI`, which uses `driver.URIOpener` when the phone has it:

- Android: `am start --user current -a android.intent.action.VIEW -d <uri>`, plus `-p <package>` when the preset has one. `internal/driver/android.viewStartArgs` builds that tail.
- iOS: `Device.OpenURI` launches the bundle with the URL, or the app iOS would pick when no bundle is set. `WDADevice.OpenURI` posts WebDriverAgent's `/url`, with `bundleId` when set.

Package names and bundle ids come from the file. None are hardcoded.

## Remote MCP and handing a task to Pinky's model

Streamable HTTP is the existing endpoint, `http://<address>/mcp`. `pinky daemon --http 0.0.0.0:7465` listens on that address. `--http-open`, or Settings › Automation › Require the agent token turned off (`RemoteOpen` in settings), serves it with no bearer token. The default still requires `pinky token`.

`run_agent_task` takes `task`, an optional `device` and optional `skills`. It starts Ask Pinky's model through the same runner as `phone_task_run`, and puts the skill catalog in the model's first message. `agent_task_status`, `agent_task_wait` and `agent_task_cancel` follow it. The text result is:

```text
status: running
task_id: t…
device: …
steps: 0
summary:
steps_detail:
screenshots:
```

The desktop app runs the model, as before. `pinky daemon` now does too, from Ask Pinky's saved service, model and key, so a client on the LAN can hand it a task while the window is closed. The key stays in the system keychain.

The model of that task sees the phone tools, `open_app_route`, the skill tools and the webview tools. It does not see `run_agent_task`, so it does not hand the task to itself.

## Embedded browser

`internal/deskweb` is the page. Settings and the sidebar **Browser** share one session: address, back, forward, reload, saved addresses in `webview.json`, and cookies of the fetch path.

- Without Chrome, Pinky fetches the HTML and lists links, buttons and fields. `webview_eval` answers `document.title`, `document.URL`, `document.body.innerText` and `document.querySelector(sel).textContent` or `.value`. A screenshot is a text file of that summary.
- **Open browser window**, or `PINKY_WEBVIEW=chrome`, starts Chrome, Chromium or Edge (`PINKY_CHROME` names the program) with a user-data directory under Pinky's data folder and a DevTools port. The person signs in in that window. The tools then use the DevTools protocol on the same page: navigation, DOM summary, click, type, scroll, arbitrary JavaScript and a PNG screenshot.

A task can use the phone and this page together: the model calls `webview_*` and `phone_*` in one run.

## Signing a Mac build with your own certificate

The identity is never hardcoded. Set one of these to the name `security find-identity -v -p codesigning` prints:

```sh
# Ad-hoc, this Mac only (the default when nothing is set):
make app

# Apple Development or Developer ID:
export PINKY_SIGN_IDENTITY="Apple Development: Your Name (TEAMID)"
make app

# The same identity on the scripts, after `make app` has produced Pinky.app
# (GOARCH is arm64 or amd64):
arch=$(go env GOARCH)
SIGN="$PINKY_SIGN_IDENTITY" scripts/macapp.sh "build/pinky/darwin-$arch/Pinky.app" build/ffmpeg

# A disk image. NOTARIZE=0 skips Apple's notary service.
SIGN="$PINKY_SIGN_IDENTITY" NOTARIZE=0 scripts/dist.sh darwin dist
PINKY_SIGN_IDENTITY="$PINKY_SIGN_IDENTITY" make release
```

`make release` refuses to run when the identity is still `-`. `CODESIGN_IDENTITY` and `MACOS_SIGN` are the other names for the same value. `macos.signingIdentity` in `cmd/pinky/mygo.json` is used when the environment does not set one.

## Tests

`go test ./internal/policy ./internal/skills ./internal/approutes ./internal/deskweb ./internal/mcp ./internal/pilot ./cmd/pinky` covers the open policy, skill import and `skill_run`, route placeholders, the fetch browser, `open_app_route` on a demo phone, the new tool list, and the daemon end to end (a payment and a password field no longer wait). Chrome itself is not started in those tests.
