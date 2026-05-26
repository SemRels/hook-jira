# hook-jira

Creates or updates Jira release metadata for the semrel version being published.

This plugin is distributed as the standalone Go binary `semrel-plugin-hook-jira`. Semrel executes the binary as a subprocess, provides plugin configuration through `SEMREL_PLUGIN_*` environment variables, provides release context through `SEMREL_*` environment variables, reads standard output, and treats exit code `0` as success and any non-zero exit code as failure. Install the binary in `~/.semrel/plugins/` or anywhere on your `$PATH`.

## Installation

```bash
go install github.com/SemRels/hook-jira/cmd/plugin@latest
```

## Configuration

```yaml
plugins:
  - name: hook-jira
    path: ~/.semrel/plugins/semrel-plugin-hook-jira
    env:
      SEMREL_PLUGIN_BASE_URL: "https://jira.example.com"
      SEMREL_PLUGIN_TOKEN: "${JIRA_TOKEN}"
      SEMREL_PLUGIN_PROJECT: "REL"
      SEMREL_PLUGIN_FIX_VERSION_TEMPLATE: "{{ .Version }}"
```

## `SEMREL_PLUGIN_*` variables

| Name | Required | Description | Default |
| --- | --- | --- | --- |
| `SEMREL_PLUGIN_BASE_URL` | Required | Base URL of the Jira instance. | None |
| `SEMREL_PLUGIN_TOKEN` | Required | Jira API token. | None |
| `SEMREL_PLUGIN_PROJECT` | Required | Jira project key. | None |
| `SEMREL_PLUGIN_FIX_VERSION_TEMPLATE` | Optional | Template used to generate the Jira fix version name. | Built-in template |

## `SEMREL_*` release context used

| Variable | Description |
| --- | --- |
| `SEMREL_VERSION` | Resolved release version for the current run. |
| `SEMREL_TAG_NAME` | Git tag name semrel will create or publish. |
| `SEMREL_NEXT_VERSION` | Next version computed by semrel for the release. |
| `SEMREL_CHANGELOG` | Generated changelog text for the release. |
| `SEMREL_DRY_RUN` | Whether semrel is running in dry-run mode. |

## Example behavior

The plugin creates or updates the Jira fix version for the release and can attach changelog details so the project release view stays in sync with semrel.

## License

Apache-2.0
