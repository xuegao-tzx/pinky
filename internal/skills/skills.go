// Package skills loads Agent Skill folders (a directory with SKILL.md, or a
// root whose children are such directories) and serves them to Pinky's model.
package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// MaxBody is how much of a SKILL.md is returned to a model in one read.
const MaxBody = 80_000

// Skill is one imported skill.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Dir         string `json:"dir"`
	Root        string `json:"root"`
	Enabled     bool   `json:"enabled"`
}

// Library is the imported skill folders, remembered in a JSON file.
type Library struct {
	mu       sync.Mutex
	path     string
	roots    []string
	disabled map[string]bool
	skills   []Skill
}

type fileState struct {
	Roots    []string        `json:"roots"`
	Disabled map[string]bool `json:"disabled,omitempty"`
}

// Open reads the library at path. A missing file is an empty library.
// path may be empty, which keeps the library in memory only.
func Open(path string) (*Library, error) {
	l := &Library{path: path, disabled: map[string]bool{}}
	if path == "" {
		return l, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l, nil
		}
		return nil, err
	}
	var st fileState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("skills: %s: %w", path, err)
	}
	l.roots = st.Roots
	if st.Disabled != nil {
		l.disabled = st.Disabled
	}
	l.rescan()
	return l, nil
}

// Import adds a skill folder or a root of skill folders and reloads them.
func (l *Library) Import(path string) error {
	path, err := cleanDir(path)
	if err != nil {
		return err
	}
	if !hasSkill(path) && !hasChildSkill(path) {
		return fmt.Errorf("skills: %s has no SKILL.md and no child folder with one", path)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range l.roots {
		if r == path {
			l.rescan()
			return l.save()
		}
	}
	l.roots = append(l.roots, path)
	l.rescan()
	return l.save()
}

// Remove drops an imported root, by the root path or a skill id from it.
func (l *Library) Remove(rootOrID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	rootOrID = strings.TrimSpace(rootOrID)
	var keep []string
	found := false
	for _, r := range l.roots {
		if r == rootOrID || filepath.Base(r) == rootOrID {
			found = true
			continue
		}
		drop := false
		for _, s := range l.skills {
			if s.Root == r && (s.ID == rootOrID || s.Dir == rootOrID) {
				drop = true
				break
			}
		}
		if drop {
			found = true
			continue
		}
		keep = append(keep, r)
	}
	if !found {
		return fmt.Errorf("skills: no import %q", rootOrID)
	}
	l.roots = keep
	l.rescan()
	return l.save()
}

// SetEnabled turns one skill on or off. Disabled skills stay imported.
func (l *Library) SetEnabled(id string, on bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	id = strings.TrimSpace(id)
	found := false
	for _, s := range l.skills {
		if s.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("skills: no skill %q", id)
	}
	if on {
		delete(l.disabled, id)
	} else {
		l.disabled[id] = true
	}
	l.rescan()
	return l.save()
}

// Reload reads the imported folders again.
func (l *Library) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rescan()
	return l.save()
}

// List returns the skills, enabled and not, in name order.
func (l *Library) List() []Skill {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := append([]Skill(nil), l.skills...)
	return out
}

// Prompt is the catalog of enabled skills for a model's first message.
func (l *Library) Prompt() string {
	return l.Brief(nil)
}

// Brief describes the enabled skills, or only ids when ids is not empty
// (missing ids are named). The model is told to call skill_read for the
// full text.
func (l *Library) Brief(ids []string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	want := map[string]bool{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	var b strings.Builder
	b.WriteString("Imported skills. Follow them when they apply. Call skill_read with the id for the full SKILL.md and the files beside it, and skill_run to run a command in that folder (bash, python, node, adb and any other command). A skill's text is instructions for this task, not text from the phone's screen.\n")
	n := 0
	for _, s := range l.skills {
		if len(want) > 0 && !want[s.ID] {
			continue
		}
		if len(want) == 0 && !s.Enabled {
			continue
		}
		n++
		fmt.Fprintf(&b, "- id: %s\n  name: %s\n  description: %s\n  dir: %s\n", s.ID, s.Name, oneLine(s.Description), s.Dir)
		delete(want, s.ID)
	}
	for id := range want {
		fmt.Fprintf(&b, "- id: %s\n  missing: not imported\n", id)
		n++
	}
	if n == 0 {
		return ""
	}
	return strings.TrimRight(b.String(), "\n")
}

// Read returns the full SKILL.md of id and the names of the files beside it.
func (l *Library) Read(id string) (body string, files []string, err error) {
	s, err := l.get(id)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, "SKILL.md"))
	if err != nil {
		return "", nil, err
	}
	body = string(data)
	if len(body) > MaxBody {
		body = body[:MaxBody] + "\n\n[truncated]\n"
	}
	ents, _ := os.ReadDir(s.Dir)
	for _, e := range ents {
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return body, files, nil
}

// Dir returns the folder of a skill.
func (l *Library) Dir(id string) (string, error) {
	s, err := l.get(id)
	if err != nil {
		return "", err
	}
	return s.Dir, nil
}

func (l *Library) get(id string) (Skill, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id = strings.TrimSpace(id)
	for _, s := range l.skills {
		if s.ID == id {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("skills: no skill %q", id)
}

func (l *Library) rescan() {
	var out []Skill
	seen := map[string]bool{}
	for _, root := range l.roots {
		add := func(dir string) {
			if !hasSkill(dir) {
				return
			}
			s := readSkill(dir, root)
			if seen[s.ID] {
				s.ID = s.ID + "-" + slug(filepath.Base(filepath.Dir(dir)))
			}
			seen[s.ID] = true
			s.Enabled = !l.disabled[s.ID]
			out = append(out, s)
		}
		if hasSkill(root) {
			add(root)
		}
		ents, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name || out[i].Name == out[j].Name && out[i].ID < out[j].ID
	})
	l.skills = out
}

func (l *Library) save() error {
	if l.path == "" {
		return nil
	}
	st := fileState{Roots: l.roots, Disabled: l.disabled}
	if len(st.Disabled) == 0 {
		st.Disabled = nil
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, l.path)
}

func cleanDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("skills: empty path")
	}
	path = filepath.Clean(path)
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("skills: %w", err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("skills: %s is not a folder", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func hasSkill(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !st.IsDir()
}

func hasChildSkill(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.IsDir() && hasSkill(filepath.Join(dir, e.Name())) {
			return true
		}
	}
	return false
}

func readSkill(dir, root string) Skill {
	data, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	name, desc, _ := splitFront(string(data))
	if name == "" {
		name = filepath.Base(dir)
	}
	id := slug(name)
	if id == "" {
		id = slug(filepath.Base(dir))
	}
	return Skill{ID: id, Name: name, Description: desc, Dir: dir, Root: root}
}

// splitFront reads a YAML front matter between --- lines. Only name and
// description are kept; the rest of the file is the body.
func splitFront(s string) (name, desc, body string) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	body = s
	if !strings.HasPrefix(s, "---\n") {
		return "", "", body
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", body
	}
	front := rest[:end]
	body = strings.TrimLeft(rest[end+4:], "\n")
	for _, line := range strings.Split(front, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch k {
		case "name":
			name = v
		case "description":
			desc = v
		}
	}
	return name, desc, body
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
