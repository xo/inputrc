# inputrc

`inputrc` parses readline `inputrc` files and holds the editing modes, the
key maps and the bind tables that readline uses. Go line editors such as
`rline` consume it.

`inputrc` only parses and describes. It does not read a terminal and it does
not edit a line. That belongs to the consumer.

## Standing rules

These hold in every `xo` repository, for every coding agent (D110).

1. Stage changes for review. Commit and push only when Ken says so.
2. Load the `simple-english` skill before you write any text that a person
   reads: a document, a code comment, an error message or a commit message.
   Follow it for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

| If you are | Read |
| --- | --- |
| writing or changing Go code | the `go-pedantry` skill. Load it first |
| writing a document, a code comment, an error message or a commit message | the `simple-english` skill. Load it first |
| answering a lint finding | the rule below, under Linting |
| adding or updating an agent skill | `CONTRIBUTING.md`, under Agent skills |
| regenerating the bind tables | `gen.go` and the comment at the top of `bind.go` |
| looking for work that is known and not done | `docs/BACKLOG.md`, when it exists |

`CONTRIBUTING.md` is the same thing for a person, and shorter.

A document that is not in that table does not exist. If you cannot find where
something is written down, it is not written down, and it is an open question.
Do not decide an open question on your own. Ask Ken.

## Hard rules

1. The module depends on the standard library and on nothing else. Its
   `go.mod` has no `require` block and must keep it that way. Do not add any
   third party package without asking Ken first.
2. Do not import `rline` or any other consumer. The dependency runs the other
   way.
3. Configuration is a value that the caller owns. Do not put a configured
   value in a package level variable.
4. Put no build constraint on an operating system or an architecture. The
   code is pure Go and builds everywhere, TinyGo included.
5. Never edit `bind.go` by hand. Change `gen.go` and run it.

## Go conventions

Wrap every error with `%w`, never `%s` or `%v`:

```go
if err != nil {
    return nil, fmt.Errorf("reading line %d: %w", n, err)
}
```

Write error messages in lower case, starting with a gerund. Do not write
"failed to" or "error". Name the object that failed.

Compare errors with `errors.Is` and `errors.As`, never by string.

Accept interfaces and return concrete structs. Keep an interface to three
methods or fewer. Define an interface where it is consumed, not where it is
implemented.

Use generics rather than `interface{}` for a container of one type.

Put `context.Context` first in every parameter list and name it `ctx`. Never
store it in a struct.

Name a package with one short lower case word. Do not stutter.

Name a receiver with one or two lower case letters, and use the same name on
every method of the type. Never write `this` or `self`.

Group struct fields by purpose, with exported fields first and a blank line
between groups.

## Linting

`.golangci.yml` holds the rules. The posture is `default: all` with a disable
list. A new linter gets a look rather than silence.

```bash
golangci-lint run ./...
```

### The rule for a lint finding

A linter that makes idiomatic Go worse is disabled, with the reason written
beside it in the configuration. Only a real defect gets a code change. Ask
which one it is before you touch the code, and if the remedy reads worse than
what it replaces, that is the answer.

A change made to quiet a linter rather than to fix something is itself a
defect. It leaves code that no reader can explain and that the next person
copies.

Never write `defer func() { _ = rows.Close() }()`. Write `defer rows.Close()`.
A deferred `Close` returns an error nothing can act on.

Never add an empty `case X:` to satisfy `exhaustive`. `exhaustive` is
disabled.

Read `golangci-lint run` output as a list of questions, not a list of tasks.

## Before you commit

Run these:

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
golangci-lint run ./...
```

`gofmt -l .` must print nothing.

Run the tests with `-count=2`, because that is what CI runs. A test body that
registers something in a registry fails the second time. Registration belongs
in `init`.

CI runs on GitHub Actions on Linux, macOS and Windows, and runs the tests
under TinyGo too. The Go version comes from `go.mod`.

## Writing documentation

A new document goes in `docs/`. Only `README.md`, `AGENTS.md`, `CLAUDE.md` and
`CONTRIBUTING.md` belong in the repository root. Add it to the table at the
top of this file, because a document nobody can find is a document nobody
reads.

A decision goes in a file of its own in `docs/decisions/` and nowhere else.
Name it `D<nnn>-<title>.md` with the next number. It opens with
`# D<n>. <title>`, a blank line, and `Status: <status>.`, where the status is
`Decided`, or `Amends D26`, or `Superseded by D24`. Add its row to
`docs/decisions/README.md`. If your decision changes an earlier one, say so in
both statuses.

Write plain English. Use short sentences and the active voice. Use `can`,
`will`, and `must`, and do not use `should`, `may`, or `might`. Do not use
semicolons or em dashes. Put the condition before the command: "If the query
returns no rows, return the empty set."

Load the `simple-english` skill before you write any text that a person
reads. Follow it for that text. Its rules include the ones above and add more,
such as no contractions and one word for one meaning. Ken asked for this.

Work that is known and not done goes in `docs/BACKLOG.md`. When an item is
done, delete it, and record anything decided in `docs/decisions/`.
