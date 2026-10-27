# Contributing to inputrc

Read [`AGENTS.md`](AGENTS.md) before you change anything. It holds the rules,
the Go conventions, the lint policy and how to run the tests. It is written for
an AI coding agent and everything in it applies to a person. `CLAUDE.md` holds
one line that imports it, for Claude Code.

Do not decide an open question on your own. Ask Ken.

## Before you send a change

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -count=2 ./...
golangci-lint run ./...
```

`gofmt -l .` must print nothing. Run the tests with `-count=2`, because that is
what CI runs.

`bind.go` is generated. Change `gen.go` and run it. Do not edit `bind.go` by
hand.

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one measured here. It writes
each skill into two folders. Codex and the other agents read
`.agents/skills/<name>`, and Claude Code reads `.claude/skills/<name>`.

To add a skill or to update one, run this in the repository root. The example
updates `simple-english`, and `skills-lock.json` holds the source for each
skill:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and says nothing. `TestSkillsAreCopies` fails
on a link, and it fails when the two folders differ.

`.claude/settings.local.json` holds the Claude Code permissions of one person.
The root `.gitignore` ignores it.
