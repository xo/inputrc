//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// keymaps are the readline keymaps, in the order bind.go lists them.
var keymaps = []string{
	"emacs", "emacs-standard", "emacs-meta", "emacs-ctlx",
	"vi", "vi-move", "vi-command", "vi-insert",
}

// terminalVars are variables that readline sets from the terminal, and so a
// dump from a terminal that has none does not list them. Both default to the
// empty string in the table, which means "use the terminal's own sequence".
var terminalVars = []string{
	"active-region-start-color",
	"active-region-end-color",
}

// dump is the defaults of one readline library.
type dump struct {
	version  string
	commands []string
	vars     []variable
	binds    map[string][]entry
}

// variable is a readline variable.
type variable struct {
	name, value string
}

// entry is a line of a keymap: a bound sequence, or a command with none.
type entry struct {
	command, seq string
	bound        bool
}

func main() {
	out := flag.String("out", "bind.go", "out")
	in := flag.String("in", "", "read a dump file instead of running gen/readline-dump.c")
	raw := flag.Bool("dump", false, "write the output without formatting it")
	flag.Parse()
	if err := run(*out, *in, *raw); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(out, in string, raw bool) error {
	var b []byte
	var err error
	if in != "" {
		b, err = os.ReadFile(in)
	} else {
		b, err = runDumper()
	}
	if err != nil {
		return err
	}
	d, err := parseDump(bytes.NewReader(b))
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	buf.WriteString(hdr)
	writeCommands(buf, d)
	writeVars(buf, d)
	writeKeymaps(buf, d)
	if raw {
		return os.WriteFile(out, buf.Bytes(), 0o644)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(out, src, 0o644)
}

// runDumper builds gen/readline-dump.c against the installed readline library
// and runs it. The environment is fixed: the meta variables follow the locale
// and the bracketed paste and active region variables follow the terminal.
func runDumper() ([]byte, error) {
	dir, err := os.MkdirTemp("", "inputrc-gen")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	exe := filepath.Join(dir, "readline-dump")
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	build := exec.Command(cc, "-o", exe, filepath.Join("gen", "readline-dump.c"), "-lreadline")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return nil, fmt.Errorf("building readline-dump: %w", err)
	}
	run := exec.Command(exe)
	run.Env = []string{"INPUTRC=/dev/null", "LC_ALL=C.UTF-8", "TERM=xterm", "HOME=/nonexistent"}
	run.Stderr = os.Stderr
	b, err := run.Output()
	if err != nil {
		return nil, fmt.Errorf("running readline-dump: %w", err)
	}
	return b, nil
}

// parseDump reads the output of readline-dump.
func parseDump(r io.Reader) (*dump, error) {
	d := &dump{binds: make(map[string][]entry)}
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()
		f := strings.SplitN(line, " ", 4)
		switch f[0] {
		case "readline":
			d.version = f[1]
		case "command":
			d.commands = append(d.commands, f[1])
		case "set":
			if len(f) < 2 {
				return nil, fmt.Errorf("bad line %q", line)
			}
			v := variable{name: f[1]}
			if len(f) > 2 {
				v.value = strings.Join(f[2:], " ")
			}
			d.vars = append(d.vars, v)
		case "bind":
			if len(f) != 4 {
				return nil, fmt.Errorf("bad line %q", line)
			}
			d.binds[f[1]] = append(d.binds[f[1]], entry{command: f[2], seq: f[3], bound: true})
		case "unbound":
			d.binds[f[1]] = append(d.binds[f[1]], entry{command: f[2]})
		default:
			return nil, fmt.Errorf("bad line %q", line)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if d.version == "" || len(d.commands) == 0 {
		return nil, fmt.Errorf("dump has no version or no commands")
	}
	return d, nil
}

func writeCommands(w io.Writer, d *dump) {
	fmt.Fprintf(w, "// Commands returns the names of every command in readline %s, bound or not.\n", d.version)
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// A consumer can use it to tell an unknown command in a user's file from a")
	fmt.Fprintln(w, "// real one.")
	fmt.Fprintln(w, "func Commands() []string {")
	fmt.Fprintln(w, "\treturn []string{")
	names := append([]string(nil), d.commands...)
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(w, "\t\t%q,\n", name)
	}
	fmt.Fprintln(w, "\t}")
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func writeVars(w io.Writer, d *dump) {
	fmt.Fprintf(w, "// DefaultVars are the default readline %s vars.\n", d.version)
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// The values are those of a UTF-8 locale on an xterm terminal. The four meta")
	fmt.Fprintln(w, "// vars (convert-meta, input-meta, output-meta and meta-flag) follow the")
	fmt.Fprintln(w, "// locale in readline, and enable-bracketed-paste and enable-active-region")
	fmt.Fprintln(w, "// follow the terminal.")
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// see: gen/readline-dump.c")
	fmt.Fprintln(w, "func DefaultVars() map[string]any {")
	fmt.Fprintln(w, "\treturn map[string]any{")
	vars := append([]variable(nil), d.vars...)
	seen := make(map[string]bool)
	for _, v := range vars {
		seen[v.name] = true
	}
	for _, name := range terminalVars {
		if !seen[name] {
			vars = append(vars, variable{name: name})
		} else {
			// the dump ran with a terminal, so blank the terminal's own value
			for i := range vars {
				if vars[i].name == name {
					vars[i].value = ""
				}
			}
		}
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].name < vars[j].name })
	for _, v := range vars {
		var val any = v.value
		typ := "q"
		switch v.value {
		case "on":
			typ, val = "t", true
		case "off":
			typ, val = "t", false
		default:
			if i, err := strconv.Atoi(v.value); err == nil {
				typ, val = "d", i
			}
		}
		fmt.Fprintf(w, "\t\t%q: %"+typ+",\n", v.name, val)
	}
	fmt.Fprintln(w, "\t}")
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func writeKeymaps(w io.Writer, d *dump) {
	fmt.Fprintf(w, "// DefaultBinds are the default readline %s bind keymaps.\n", d.version)
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// see: gen/readline-dump.c")
	fmt.Fprintln(w, "func DefaultBinds() map[string]map[string]Bind {")
	fmt.Fprintln(w, "\treturn map[string]map[string]Bind {")
	for _, keymap := range keymaps {
		fmt.Fprintf(w, "\t\t%q: {\n", keymap)
		m := make(map[string]bool)
		for _, e := range d.binds[keymap] {
			if !e.bound {
				fmt.Fprintf(w, "\t\t\t// %s (not bound)\n", e.command)
				continue
			}
			z := e.seq
			switch b := []byte(z); {
			case !utf8.Valid(b):
				z = `string([]byte{` + encode(b) + `})`
			case z == "`":
				z = "\"`\""
			default:
				z = "`" + z + "`"
			}
			if !m[z] {
				fmt.Fprintf(w, "\t\t\tUnescape(%s): {%q, false},\n", z, e.command)
			} else {
				fmt.Fprintf(w, "\t\t\t// Unescape(%s): {%q, false}, // DUPLICATE KEY\n", z, e.command)
			}
			m[z] = true
		}
		fmt.Fprintln(w, "\t\t},")
	}
	fmt.Fprintln(w, "\t}")
	fmt.Fprintln(w, "}")
}

func encode(b []byte) string {
	var s []string
	for _, c := range b {
		s = append(s, fmt.Sprintf("0x%x", c))
	}
	return strings.Join(s, ", ")
}

const hdr = `package inputrc

// Code generated by gen.go. DO NOT EDIT.

`
