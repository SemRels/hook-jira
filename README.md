# hook-jira

`hook-jira` is a SemRel hook plugin that transitions Jira issues referenced by release commits and comments with release details.

## Configuration

Environment variables:

- `JIRA_BASE_URL`
- `JIRA_EMAIL`
- `JIRA_API_TOKEN`

## Behavior

- Parses commit messages for footers like `Fixes PROJ-123`
- Chooses the `Released` transition when available, otherwise `Done`
- Adds a release comment to each transitioned issue

## Development

```bash
go mod tidy
go build ./...
go test ./...
```
