# Chronos

A command-line time tracking tool for Clockify that makes logging time entries quick and easy.

## Features

- 🕐 **Quick Time Logging**: Log time entries with duration and task description in one command
- ⚡ **Simple CLI**: Easy-to-use command-line interface with aliases
- 🔧 **Environment-based Configuration**: Secure API key and configuration management

## Installation

### Prerequisites

- Go 1.24.2 or later
- A Clockify account and API key

### Build from Source

```bash
git clone https://github.com/andrejsoucek/chronos.git
cd chronos
go build -o chronos cmd/main.go
```

## Configuration

Create a `.env` file in your `$HOME/.chronos` directory with the following configuration:

```env
CLOCKIFY_API_KEY=your_api_key_here
CLOCKIFY_BASE_URL=https://api.clockify.me/api/v1/workspaces/YOUR_WORKSPACE_ID
CLOCKIFY_USER_URL=https://api.clockify.me/api/v1/user
CLOCKIFY_WORKSPACE=your_workspace_id
CLOCKIFY_DEFAULT_PROJECT=your_default_project_id
GITLAB_ACCESS_TOKEN=
GITLAB_BASE_URL=https://gitlab.com/api/v4/
GITLAB_USER_ID=123456789
LINEAR_API_KEY=
LINEAR_BASE_URL=https://api.linear.app/graphql
```

### Getting Your Configuration Values

1. **Clockify API Key**: Get your API key from [Clockify Settings](https://clockify.me/user/settings)
2. **Workspace ID**: Run `chronos workspace` to get your workspace information
3. **Project ID**: You can find project IDs by opening a [Project](https://app.clockify.me/projects) and copying the ID from the URL
4. **Gitlab Access Token**: Get your API key from [Gitlab Settings](https://gitlab.com/-/user_settings/personal_access_tokens)
5. **Gitlab User ID**: Open `https://gitlab.com/api/v4/users?username=YOUR_USERNAME` in your browser.
6. **Linear API Key**: Get your API key from [Linear Settings](https://linear.app/aristone/settings/account/security/api-keys)

## Available Commands

| Command | Alias | Description |
|---------|--------|-------------|
| `workspace` | `ws` | Get workspace information |
| `log` | `l` | Log a time entry |
| `report` | `r` | Show an editable month report |

## Usage

### Get Workspace Information

```bash
chronos workspace
# or use the alias
chronos ws
```

This command returns formatted JSON with your workspace details.

### Log Time Entry

```bash
chronos log <duration> <task_description>
# or use the alias
chronos l <duration> <task_description>
```

**Examples:**

```bash
# Log 2 hours for debugging
chronos log 2h "Fixed authentication bug"

# Log 30 minutes for a meeting
chronos log 30m "Sprint planning meeting"

# Log 1 hour and 15 minutes for code review
chronos log 1h15m "Code review for PR #123"
```

**Supported Duration Formats:**

- `2h` - 2 hours
- `30m` - 30 minutes
- `1h30m` - 1 hour and 30 minutes
- `45s` - 45 seconds

### Report

```bash
chronos report
# or use the alias
chronos r
```

### Help

```bash
chronos --help
chronos log --help
chronos workspace --help
```

## How It Works

1. **Time Logging**: When you log time, Chronos calculates the start and end times based on the current time and the duration you specify
2. **Automatic Rounding**: Times are automatically rounded to the nearest 30-minute interval
3. **Billable by Default**: All logged entries are marked as billable
4. **Project Association**: Time entries are associated with your default project specified in the configuration

## Development

### Running from Source

```bash
go run cmd/main.go workspace
go run cmd/main.go log 1h30m "Development work"
```

### Building

```bash
go build -o chronos cmd/main.go
```
