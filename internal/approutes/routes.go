// Package approutes keeps the app deep-link presets Pinky can open on
// Android and iOS: a name, a URL scheme, a package name, a bundle id and
// route shortcuts. Nothing is hardcoded; the file is the person's.
package approutes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Route is a shortcut under an app's scheme.
type Route struct {
	Name string `yaml:"name" json:"name"`
	// Path is a URI or a path. "{scheme}" is the app's scheme prefix.
	// "{name}" placeholders are filled from the call's parameters.
	Path string `yaml:"path" json:"path"`
}

// App is one preset.
type App struct {
	Name           string  `yaml:"name" json:"name"`
	Scheme         string  `yaml:"scheme" json:"scheme"`
	AndroidPackage string  `yaml:"android_package" json:"android_package"`
	IOSBundleID    string  `yaml:"ios_bundle_id" json:"ios_bundle_id"`
	Routes         []Route `yaml:"routes,omitempty" json:"routes,omitempty"`
}

// File is the YAML document.
type File struct {
	Apps []App `yaml:"apps"`
}

// Catalog is the app route file.
type Catalog struct {
	path string
	file File
}

// Open reads path. A missing file is an empty catalog.
func Open(path string) (*Catalog, error) {
	c := &Catalog{path: path}
	if path == "" {
		return c, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, &c.file); err != nil {
		return nil, fmt.Errorf("app routes: %s: %w", path, err)
	}
	return c, nil
}

// List returns the presets.
func (c *Catalog) List() []App {
	out := append([]App(nil), c.file.Apps...)
	return out
}

// Add puts a preset at the end, replacing one with the same name, and
// saves the file when the catalog has a path.
func (c *Catalog) Add(a App) error {
	a.Name = strings.TrimSpace(a.Name)
	a.Scheme = strings.TrimSpace(a.Scheme)
	a.AndroidPackage = strings.TrimSpace(a.AndroidPackage)
	a.IOSBundleID = strings.TrimSpace(a.IOSBundleID)
	if a.Name == "" {
		return errors.New("app routes: a preset needs a name")
	}
	if a.Scheme == "" && len(a.Routes) == 0 {
		return errors.New("app routes: a preset needs a URL scheme or a route")
	}
	var keep []App
	for _, old := range c.file.Apps {
		if !strings.EqualFold(old.Name, a.Name) {
			keep = append(keep, old)
		}
	}
	c.file.Apps = append(keep, a)
	return c.save()
}

// Remove drops a preset by name.
func (c *Catalog) Remove(name string) error {
	var keep []App
	found := false
	for _, a := range c.file.Apps {
		if strings.EqualFold(a.Name, name) {
			found = true
			continue
		}
		keep = append(keep, a)
	}
	if !found {
		return fmt.Errorf("app routes: no preset %q", name)
	}
	c.file.Apps = keep
	return c.save()
}

// Target is a URI to open and which app id to pass on each platform.
type Target struct {
	URI            string
	AndroidPackage string
	IOSBundleID    string
	App            string
}

// Resolve builds a URI. app is a preset name, a package or a bundle id.
// route is a shortcut name or a path. uri, when set, is used as it is.
// params fills {name} placeholders.
func (c *Catalog) Resolve(app, route, uri string, params map[string]string) (Target, error) {
	if uri = strings.TrimSpace(uri); uri != "" {
		t := Target{URI: fill(uri, "", params)}
		if a, ok := c.find(app); ok {
			t.App = a.Name
			t.AndroidPackage = a.AndroidPackage
			t.IOSBundleID = a.IOSBundleID
		}
		return t, nil
	}
	a, ok := c.find(app)
	if !ok {
		return Target{}, fmt.Errorf("app routes: no preset %q", app)
	}
	t := Target{App: a.Name, AndroidPackage: a.AndroidPackage, IOSBundleID: a.IOSBundleID}
	route = strings.TrimSpace(route)
	if route == "" {
		t.URI = strings.TrimSpace(a.Scheme)
		if t.URI == "" {
			return Target{}, fmt.Errorf("app routes: %s has no scheme", a.Name)
		}
		t.URI = fill(t.URI, a.Scheme, params)
		return t, nil
	}
	for _, r := range a.Routes {
		if strings.EqualFold(r.Name, route) {
			t.URI = fill(r.Path, a.Scheme, params)
			if t.URI == "" {
				return Target{}, fmt.Errorf("app routes: route %s of %s is empty", r.Name, a.Name)
			}
			return t, nil
		}
	}
	// A path or a full URI that is not a shortcut name.
	if strings.Contains(route, "://") {
		t.URI = fill(route, a.Scheme, params)
		return t, nil
	}
	base := strings.TrimSpace(a.Scheme)
	if base == "" {
		return Target{}, fmt.Errorf("app routes: %s has no scheme for %q", a.Name, route)
	}
	if strings.HasPrefix(route, "/") && strings.HasSuffix(base, "/") {
		route = strings.TrimPrefix(route, "/")
	}
	t.URI = fill(base+route, a.Scheme, params)
	return t, nil
}

func (c *Catalog) find(ref string) (App, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return App{}, false
	}
	for _, a := range c.file.Apps {
		if strings.EqualFold(a.Name, ref) || a.AndroidPackage == ref || a.IOSBundleID == ref {
			return a, true
		}
	}
	return App{}, false
}

func (c *Catalog) save() error {
	if c.path == "" {
		return nil
	}
	data, err := yaml.Marshal(c.file)
	if err != nil {
		return err
	}
	head := []byte("# App route presets. name, scheme, android_package and ios_bundle_id are yours.\n# A route path may use {scheme} and {param} placeholders.\n")
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, append(head, data...), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

func fill(path, scheme string, params map[string]string) string {
	path = strings.ReplaceAll(path, "{scheme}", scheme)
	for k, v := range params {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	return path
}
