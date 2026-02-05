package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gunnarrb/jql-to-plan/internal/config"
	"github.com/spf13/cobra"
)

// configTemplate is the default configuration file content
const configTemplate = `# Jira Configuration
jira_url: "https://your-domain.atlassian.net"
jira_pat: "your-personal-access-token"

# Optional: Custom Field ID for Effort (e.g. customfield_10105)
# This is required for accurate effort estimation in the Gantt chart.
# effort_custom_field_id: "10105"

# Optional: Custom Field ID for Epic Link (e.g. customfield_11000)
# This is required for grouping tasks by Epic.
# epic_link_custom_field_id: "11000"
`

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Create and edit the configuration file",
	Long:  `Creates a new configuration file at $HOME/.jql-to-plan.yaml if one does not exist, and opens it in your default editor.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, err := config.GetConfigPath()
		if err != nil {
			fmt.Printf("Error getting config path: %v\n", err)
			os.Exit(1)
		}

		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Create the file with template content
			if err := os.WriteFile(configPath, []byte(configTemplate), 0600); err != nil {
				fmt.Printf("Error creating config file at %s: %v\n", configPath, err)
				os.Exit(1)
			}
			fmt.Printf("Created new configuration file at: %s\n", configPath)
		} else {
			fmt.Printf("Configuration file already exists at: %s\n", configPath)
		}

		// Open in editor
		openInEditor(configPath)
	},
}

func init() {
	// No flags needed for now
}

func openInEditor(path string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Fallback defaults based on OS
		switch runtime.GOOS {
		case "windows":
			editor = "notepad"
		case "darwin", "linux":
			editor = "vi" // reliable fallback usually
		default:
			fmt.Printf("No $EDITOR set. Please edit the file manually at: %s\n", path)
			return
		}
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Opening config file in %s...\n", editor)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error opening editor: %v\n", err)
		fmt.Printf("Please edit the file manually at: %s\n", path)
	}
}
