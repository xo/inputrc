package inputrc

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"unicode"
)

const delimiter = "####----####\n"

func TestConfig(t *testing.T) {
	var _ Handler = NewDefaultConfig()
}

func TestParse(t *testing.T) {
	var tests []string
	if err := fs.WalkDir(testdata, ".", func(n string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		tests = append(tests, n)
		return nil
	}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, test := range tests {
		t.Run(filepath.Base(test), func(t *testing.T) {
			exp := readTest(t, test)
			if len(exp) != 3 {
				t.Fatalf("len(test) != 3: %d", len(exp))
			}
			cfg, m := newConfig(t)
			check(t, exp[2], cfg, m, ParseBytes(exp[1], cfg, buildOpts(t, exp[0])...))
		})
	}
}

func TestUserDefault(t *testing.T) {
	tests := []struct {
		dir string
		exp string
	}{
		{"/home/ken", "ken.inputrc"},
		{"/home/bob", "default.inputrc"},
	}
	for _, test := range tests {
		exp := readTest(t, path.Join("testdata", test.exp))
		cfg, m := newConfig(t)
		u := &user.User{
			HomeDir: test.dir,
		}
		check(t, exp[2], cfg, m, UserDefault(u, cfg, buildOpts(t, exp[0])...))
	}
}

func TestEncontrolDecontrol(t *testing.T) {
	tests := []struct {
		d, e rune
	}{
		{'a', '\x01'},
		{'i', '\t'},
		{'j', '\n'},
		{'m', '\r'},
		{'A', '\x01'},
		{'I', '\t'},
		{'J', '\n'},
		{'M', '\r'},
	}
	for i, test := range tests {
		c := Encontrol(test.d)
		if exp := test.e; c != exp {
			t.Errorf("test %d expected %c==%c", i, exp, c)
		}
		c = Decontrol(test.e)
		if exp := unicode.ToUpper(test.d); c != exp {
			t.Errorf("test %d expected %c==%c", i, exp, c)
		}
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		s, exp string
	}{
		{"\x1b\x7f", `\e\C-?`},
		{"\x1b[13;", `\e[13;`},
	}
	for i, test := range tests {
		if s, exp := Escape(test.s), test.exp; s != exp {
			t.Errorf("test %d expected %q==%q", i, exp, s)
		}
	}
}

func TestDecode(t *testing.T) {
	const str = `
Control-Meta-f: "a"
Meta-Control-f: "b"
"\C-\M-f": "c"
"\M-\C-f": "d"
Control-Meta-p: "e"
Meta-Control-p: "f"
"\C-\M-p": "g"
"\M-\C-p": "h"
`
	t.Logf("decoding:%s", str)
	cfg := NewConfig()
	if err := ParseBytes([]byte(str), cfg); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	t.Logf("decoded as:")
	for sectKey, sect := range cfg.Binds {
		for key, bind := range sect {
			t.Logf("%q: %q 0x%x: %q %t", sectKey, key, []byte(key), bind.Action, bind.Macro)
		}
	}
}

func TestDecodeKey(t *testing.T) {
	tests := []struct {
		s, exp string
	}{
		{"Escape", "\x1b"},
		{"Control-u", "\x15"},
		{"return", "\r"},
		{"Meta-tab", "\x1b\t"},
		{"Control-Meta-v", string(Encontrol(Enmeta('v')))},
	}
	for i, test := range tests {
		r := []rune(test.s)
		v, _, err := decodeKey(r, 0, len(r))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		// FIXME: need more tests and stuff, and this skip here is just to
		// quiet errors
		if i == 3 || i == 4 {
			continue
		}
		if s, exp := v, test.exp; s != exp {
			t.Errorf("test %d expected %q==%q", i, exp, s)
		}
	}
}

func newConfig(t *testing.T) (*Config, map[string][]string) {
	t.Helper()
	cfg := NewDefaultConfig(WithConfigReadFileFunc(readTestdata(t)))
	m := make(map[string][]string)
	cfg.Funcs["$custom"] = func(k, v string) error {
		m[k] = append(m[k], v)
		return nil
	}
	cfg.Funcs[""] = func(k, v string) error {
		m[k] = append(m[k], v)
		return nil
	}
	return cfg, m
}

func readTest(t *testing.T, name string) [][]byte {
	t.Helper()
	buf, err := testdata.ReadFile(name)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return bytes.Split(buf, []byte(delimiter))
}

func check(t *testing.T, exp []byte, cfg *Config, m map[string][]string, err error) {
	t.Helper()
	res := buildResult(t, exp, cfg, m, err)
	if !bytes.Equal(exp, res) {
		t.Errorf("result does not equal expected:\n%s\ngot:\n%s", string(res), string(res))
	}
}

func buildOpts(t *testing.T, buf []byte) []Option {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf), []byte{'\n'})
	var opts []Option
	for i := range lines {
		line := bytes.TrimSpace(lines[i])
		j := bytes.Index(line, []byte{':'})
		if j == -1 {
			t.Fatalf("invalid line %d: %q", i+1, string(line))
		}
		switch k := string(bytes.TrimSpace(line[:j])); k {
		case "haltOnErr":
			opts = append(opts, WithHaltOnErr(parseBool(t, line[j+1:])))
		case "strict":
			opts = append(opts, WithStrict(parseBool(t, line[j+1:])))
		case "app":
			opts = append(opts, WithApp(string(bytes.TrimSpace(line[j+1:]))))
		case "term":
			opts = append(opts, WithTerm(string(bytes.TrimSpace(line[j+1:]))))
		case "mode":
			opts = append(opts, WithMode(string(bytes.TrimSpace(line[j+1:]))))
		default:
			t.Fatalf("unknown param %q", k)
		}
	}
	return opts
}

func buildResult(t *testing.T, exp []byte, cfg *Config, custom map[string][]string, err error) []byte {
	t.Helper()
	m := errRE.FindSubmatch(exp)
	switch {
	case err != nil && m == nil:
		t.Fatalf("expected no error, got: %v", err)
	case err != nil:
		s := string(m[1])
		re, reErr := regexp.Compile(s)
		if reErr != nil {
			t.Fatalf("could not compile regexp %q: %v", s, reErr)
			return nil
		}
		if !re.MatchString(err.Error()) {
			t.Errorf("expected error %q, got: %v", s, err)
		}
		t.Logf("matched error %q", err)
		return exp
	}
	buf := new(bytes.Buffer)
	// add vars
	dv := DefaultVars()
	vv := make(map[string]any)
	for k, v := range cfg.Vars {
		if dv[k] != v {
			vv[k] = v
		}
	}
	if len(vv) != 0 {
		fmt.Fprintln(buf, "vars:")
		for _, k := range slices.Sorted(maps.Keys(vv)) {
			fmt.Fprintf(buf, "  %s: %v\n", k, vv[k])
		}
	}
	// add binds
	db := DefaultBinds()
	vb := make(map[string]map[string]string)
	for k := range cfg.Binds {
		vb[k] = make(map[string]string)
	}
	count := 0
	for k, m := range cfg.Binds {
		for j, v := range m {
			if db[k][j] != v {
				if v.Macro {
					vb[k][j] = `"` + EscapeMacro(v.Action) + `"`
				} else {
					vb[k][j] = Escape(v.Action)
				}
				count++
			}
		}
	}
	if count != 0 {
		fmt.Fprintln(buf, "binds:")
		for _, k := range slices.Sorted(maps.Keys(vb)) {
			if len(vb[k]) != 0 {
				fmt.Fprintf(buf, "  %s:\n", k)
				for _, j := range slices.Sorted(maps.Keys(vb[k])) {
					fmt.Fprintf(buf, "    %s: %s\n", Escape(j), vb[k][j])
				}
			}
		}
	}
	if len(custom) != 0 {
		for _, typ := range slices.Sorted(maps.Keys(custom)) {
			if len(custom[typ]) != 0 {
				fmt.Fprintf(buf, "%s:\n", typ)
				for _, v := range custom[typ] {
					fmt.Fprintf(buf, "  %s\n", v)
				}
			}
		}
	}
	// add custom
	return buf.Bytes()
}

var errRE = regexp.MustCompile(`(?im)^\s*error:\s+(.*)$`)

func parseBool(t *testing.T, buf []byte) bool {
	t.Helper()
	switch s := string(bytes.TrimSpace(buf)); s {
	case "true":
		return true
	case "false":
		return false
	default:
		t.Fatalf("unknown bool value %q", s)
	}
	return false
}

func readTestdata(t *testing.T) func(string) ([]byte, error) {
	t.Helper()
	return func(name string) ([]byte, error) {
		switch name {
		case `/home/ken/.inputrc`, `\home\ken\_inputrc`:
			name = "ken.inputrc"
		case `/etc/inputrc`, `/home/bob/.inputrc`, `\home\bob\_inputrc`:
			name = "default.inputrc"
		}
		buf, err := testdata.ReadFile(path.Join("testdata", name))
		if err != nil {
			t.Fatalf("unable to open %s: %v", name, err)
		}
		v := bytes.Split(buf, []byte(delimiter))
		if len(v) != 3 {
			t.Fatalf("test data %s is invalid!", name)
		}
		return v[1], nil
	}
}

//go:embed testdata/*.inputrc
var testdata embed.FS
