package inputrc

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"testing"
)

func TestIfTests(t *testing.T) {
	tests := []struct {
		test string
		opts []Option
		exp  bool
	}{
		{"version >= 8.0", nil, true},
		{"version >= 8.3", nil, true},
		{"version > 8.3", nil, false},
		{"version == 8", nil, false},
		{"version != 7.0", nil, true},
		{"version<9", nil, true},
		{"version <= 8.2", []Option{WithVersion("8.2")}, true},
		{"version >= 8.0", []Option{WithVersion("7.1")}, false},
		{"editing-mode == emacs", nil, true},
		{"editing-mode == vi", nil, false},
		{"editing-mode != vi", nil, true},
		{"editing-mode=emacs", nil, true},
		{"enable-bracketed-paste == on", nil, true},
		{"no-such-variable == on", nil, false},
		{"term=xterm", []Option{WithTerm("xterm-256color")}, true},
		{"term=xterm-256color", []Option{WithTerm("xterm-256color")}, true},
		{"term=screen", []Option{WithTerm("xterm-256color")}, false},
		{"bash", []Option{WithApp("bash")}, true},
		{"bash # comment", []Option{WithApp("bash")}, true},
		{"Bash", []Option{WithApp("bash")}, true},
		{"bash", []Option{WithApp("Bash")}, true},
		{"Bash", []Option{WithApp("Bash")}, true},
		{"Bas", []Option{WithApp("Bash")}, false},
	}
	for _, test := range tests {
		cfg := NewDefaultConfig()
		src := "$if " + test.test + "\nset matched on\n$endif\n"
		if err := ParseBytes([]byte(src), cfg, test.opts...); err != nil {
			t.Fatalf("%q: expected no error, got: %v", test.test, err)
		}
		if got := cfg.GetBool("matched"); got != test.exp {
			t.Errorf("%q: expected %t, got %t", test.test, test.exp, got)
		}
	}
}

func TestMetaIsEscPrefix(t *testing.T) {
	tests := map[string]string{
		`\M-a`:    "\x1ba",
		`\M-\C-x`: "\x1b\x18",
		`\C-\M-x`: "\x1b\x18",
		`\M-\C-?`: "\x1b\x1f",
	}
	for s, exp := range tests {
		if got := Unescape(s); got != exp {
			t.Errorf("Unescape(%q): expected %q, got %q", s, exp, got)
		}
	}
	cfg := NewConfig()
	if err := ParseBytes([]byte("Meta-a: foo\n"), cfg); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, ok := cfg.Binds["emacs"]["\x1ba"]; !ok {
		t.Errorf("expected Meta-a to bind Esc a, got: %v", cfg.Binds)
	}
}

type locConfig struct {
	*Config

	name string
	line int
	seen []string
}

func (c *locConfig) Locate(name string, line int) { c.name, c.line = name, line }

func (c *locConfig) Set(name string, value any) error {
	c.seen = append(c.seen, name+"@"+c.name+":"+strconv.Itoa(c.line))
	return c.Config.Set(name, value)
}

func TestLocator(t *testing.T) {
	cfg := &locConfig{Config: NewDefaultConfig()}
	src := "# comment\nset no-such-variable on\n\nset convert-meta on\n"
	if err := ParseBytes([]byte(src), cfg, WithName("x.inputrc")); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	exp := []string{"no-such-variable@x.inputrc:2", "convert-meta@x.inputrc:4"}
	if !slices.Equal(cfg.seen, exp) {
		t.Errorf("expected %v, got %v", exp, cfg.seen)
	}
}

func TestCommands(t *testing.T) {
	cmds := Commands()
	if len(cmds) != 144 {
		t.Errorf("expected 144 commands, got %d", len(cmds))
	}
	if !slices.IsSorted(cmds) {
		t.Error("expected commands to be sorted")
	}
	for _, binds := range DefaultBinds() {
		for seq, b := range binds {
			if b.Macro {
				continue
			}
			if _, ok := slices.BinarySearch(cmds, b.Action); !ok {
				t.Errorf("default bind %q names unknown command %q", Escape(seq), b.Action)
			}
		}
	}
}

func TestSetValues(t *testing.T) {
	tests := []struct {
		src  string
		name string
		exp  any
	}{
		{"set completion-query-items 3\n", "completion-query-items", 3},
		{"set isearch-terminators x\n", "isearch-terminators", "x"},
		{"set isearch-terminators \"\\C-a\"\n", "isearch-terminators", `\C-a`},
		{"set isearch-terminators '\\C-a'\n", "isearch-terminators", `\C-a`},
		{"set comment-begin \"# \"\n", "comment-begin", "# "},
		{"set bell-style none # comment\n", "bell-style", "none"},
	}
	for _, test := range tests {
		cfg := NewDefaultConfig()
		if err := ParseBytes([]byte(test.src), cfg, WithHaltOnErr(true)); err != nil {
			t.Fatalf("%q: expected no error, got: %v", test.src, err)
		}
		if got := cfg.Vars[test.name]; got != test.exp {
			t.Errorf("%q: expected %#v, got %#v", test.src, test.exp, got)
		}
	}
}

func TestKeymapAfterSet(t *testing.T) {
	tests := []struct {
		src    string
		keymap string
		seq    string
	}{
		{"set keymap emacs-meta\n\"a\": foo\n", "emacs-meta", "a"},
		{"set editing-mode vi\n\"\\C-xq\": kill-line\n", "vi-insert", "\x18q"},
		{"set editing-mode vi\nset editing-mode emacs\n\"\\C-xq\": kill-line\n", "emacs", "\x18q"},
		{"d: backward-char\n", "emacs", "d"},
		{"\"d\": x\n", "emacs", "d"},
	}
	for _, test := range tests {
		cfg := NewDefaultConfig()
		if err := ParseBytes([]byte(test.src), cfg, WithHaltOnErr(true)); err != nil {
			t.Fatalf("%q: expected no error, got: %v", test.src, err)
		}
		if _, ok := cfg.Binds[test.keymap][test.seq]; !ok {
			t.Errorf("%q: expected a bind in %s for %q", test.src, test.keymap, test.seq)
		}
	}
}

func TestUserDefaultInputrc(t *testing.T) {
	home := ".inputrc"
	if runtime.GOOS == "windows" {
		home = "_inputrc"
	}
	files := map[string]string{
		"/custom/inputrc":                "set bell-style none\n",
		filepath.Join("/home/ken", home): "set bell-style visible\n",
	}
	read := func(name string) ([]byte, error) {
		if s, ok := files[name]; ok {
			return []byte(s), nil
		}
		return nil, os.ErrNotExist
	}
	u := &user.User{HomeDir: "/home/ken"}
	tests := []struct {
		env string
		exp string
	}{
		// INPUTRC names a file that exists, so only it is read
		{"/custom/inputrc", "none"},
		// INPUTRC names a missing file, so nothing is read and there is no fallback
		{"/missing/inputrc", "audible"},
		// INPUTRC is empty, so the home file is read
		{"", "visible"},
	}
	for _, test := range tests {
		t.Setenv("INPUTRC", test.env)
		cfg := NewDefaultConfig(WithConfigReadFileFunc(read))
		if err := UserDefault(u, cfg); err != nil {
			t.Fatalf("INPUTRC=%q: expected no error, got: %v", test.env, err)
		}
		if got := cfg.GetString("bell-style"); got != test.exp {
			t.Errorf("INPUTRC=%q: expected bell-style %q, got %q", test.env, test.exp, got)
		}
	}
}
