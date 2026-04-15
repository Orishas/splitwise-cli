# splitwise

A fast CLI for [Splitwise](https://www.splitwise.com). Manage groups, expenses, balances, and settlements from your terminal.

## Install

### From source (recommended)

Requires Go 1.22+.

```bash
git clone <this-repo-url>
cd splitwise-cli
go build -o splitwise .
./splitwise --version
```

Move the binary somewhere on your `PATH`, e.g. `mv splitwise /usr/local/bin/`.

### Pre-built binaries (requires your own fork)

The Homebrew tap and `go install` paths below reference the placeholder org
`example` (see `go.mod`, `.goreleaser.yml`, `Formula/splitwise.rb`). To use them
you need to fork this repo, replace `example` with your GitHub user/org in those
files, and publish releases via [GoReleaser](https://goreleaser.com/).

```bash
# After you've published your own fork + tap:
brew tap <your-user>/tap
brew install splitwise

# Or:
go install github.com/<your-user>/splitwise-cli@latest
```

## Setup

1. **Register a Splitwise app** at [secure.splitwise.com/apps](https://secure.splitwise.com/apps)
   - Set the callback URL to `http://localhost/callback` (the CLI listens on a random local port and does not rely on a fixed port).
2. **Authenticate:**

```bash
splitwise auth
```

You'll be prompted for your Client ID and Client Secret. The CLI opens your browser for OAuth, then stores the token at `~/.config/splitwise-cli/auth.json`.

> **Note:** Splitwise ignores `redirect_uri` in OAuth requests and always uses the registered callback URL. If the registered URL is `http://localhost/callback` the browser will land on a page that can't connect; copy the `code` from the URL and complete the exchange manually with `curl`.

## Usage

```bash
# Who am I?
splitwise me

# Groups & balances
splitwise groups
splitwise group "Household"

# Expenses
splitwise expenses list --group "Household" --limit 20
splitwise expenses list --after 2026-01-01
splitwise expenses list --all                      # include deleted
splitwise expenses show 123456
splitwise expenses create "Dinner" 85.50 --group "Household"
splitwise expenses create "Groceries" 42.00 --paid-by "MemberB"
splitwise expenses create "Utilities" 254.80 --split "exact:MemberA:152.88,MemberB:101.92"
splitwise expenses create "Dinner last week" 50 --date 2026-04-08 --details "Thai place"
splitwise expenses delete 123456
splitwise expenses restore 123456

# Balances
splitwise balances
splitwise balances --group "Household"

# Friends
splitwise friends

# Settle up
splitwise settle "MemberB" --group "Household"

# Set defaults
splitwise config set default_group "Household"
splitwise config set default_currency USD
splitwise config show
```

## Output

| Flag | Description |
|------|-------------|
| `--json` | Raw JSON |
| `--quiet` | IDs/amounts only (for scripting) |
| `--no-color` | No ANSI colors (also respects `NO_COLOR`) |

```bash
splitwise expenses list --quiet | head -5
splitwise balances --json | jq '.[] | select(.amount != "0.00")'
```

## AI Agent Integration

This CLI ships with an [OpenClaw](https://github.com/openclaw/openclaw) / [Gemini CLI](https://github.com/google-gemini/gemini-cli) skill file. Drop `skills/splitwise/SKILL.md` into your agent's skills directory to let your AI assistant manage Splitwise expenses conversationally.

## Configuration

| File | Purpose |
|------|---------|
| `~/.config/splitwise-cli/auth.json` | OAuth token |
| `~/.config/splitwise-cli/config.json` | User preferences |

## License

MIT
