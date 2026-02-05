# jql-to-plan

A CLI tool to fetch Jira tickets via JQL and convert them into an OmniPlan project file (`.oplx`).

## Description

`jql-to-plan` allows you to quickly visualize a set of Jira issues in OmniPlan. It executes a JQL query against your Jira instance, retrieves the matching tickets, and generates an OmniPlan file package containing the tasks, along with their estimated effort and assignments.

## Features

-   **JQL Integration**: Fetch tickets using any valid JQL query.
-   **Effort Mapping**: Maps a designated Jira custom field (representing effort in days) to OmniPlan task effort.
-   **Resource Assignment**: Maps Jira assignees to OmniPlan resources.
-   **OmniPlan Package Generation**: Creates a complete `.oplx` directory structure.
-   **Configurable**: Easy configuration via environment variables or a config file.



## Installation

### From Source

Ensure you have Go installed on your machine.

```bash
git clone https://github.com/gunnarrb/jql-to-plan.git
cd jql-to-plan
go install
```

### Configuration

The tool requires your Jira instance URL and a Personal Access Token (PAT) to authenticate.

To set up your configuration initially, run:

```bash
jql-to-plan config
```

This command will:
1.  Create a configuration file at `~/.jql-to-plan.yaml` if it doesn't represent.
2.  Open the file in your default editor (from `$EDITOR` environment variable).

You must then edit the file to provide your `jira_url` and `jira_pat`. You should also uncomment and set the `effort_custom_field_id` to ensure effort is correctly mapped.

### Manual Configuration

You can also manually create the configuration file `~/.jql-to-plan.yaml`:

```yaml
jira_url: "https://yourdomain.atlassian.net"
jira_pat: "your-personal-access-token"
effort_custom_field_id: "10105" # Optional, but recommended for accurate effort
epic_link_custom_field_id: "10106" # Required if using the --epic-group flag
```

Alternatively, you can provide these via environment variables, though using the config file is recommended for the custom field IDs.

*   `JIRA_URL` and `JIRA_PAT`

## Usage

Run the tool by providing a **Project Name** and a **JQL Query**.

```bash
jql-to-plan [project_name] [JQL] [flags]
```

### Flags

-   `--epic-group`, `-e`: Group tasks by their Epic. Requires `epic_link_custom_field_id` to be set in the configuration.
-   `--milestone-done`, `-m`: Add a final "Done" milestone to the project plan. If used with `--epic-group`, it will depend on all adjacent Epic milestones. Otherwise, it depends on all other milestones in the plan.

### Example

```bash
jql-to-plan Q1Planning "project = PROJ AND type in (Story, Task) AND status != Done"
```

This command will:
1.  Connect to Jira using your configured credentials.
2.  Fetch issues matching the JQL query.
3.  Create a plan named `Q1Planning.oplx`.

You can then open the `Q1Planning.oplx` project directly with OmniPlan.

## Helper Scripts

The `scripts/` directory contains helper scripts to assist with configuration:

*   **`fetch_custom_fields.sh`**: Fetches all fields from your Jira instance. Use this to find the ID of the custom field you want to use for "Effort" (e.g., "Story Points").
    *   Usage: `export JIRA_URL=...; export JIRA_PAT=...; ./scripts/fetch_custom_fields.sh`
*   **`fetch_issue_links.sh`**: Fetches all issue link types. Use this to identify the link type IDs for dependency mapping.
    *   Usage: `export JIRA_URL=...; export JIRA_PAT=...; ./scripts/fetch_issue_links.sh`
