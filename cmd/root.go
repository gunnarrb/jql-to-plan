package cmd

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/gunnarrb/jql-to-plan/internal/config"
	"github.com/gunnarrb/jql-to-plan/internal/jira"
	"github.com/gunnarrb/jql-to-plan/internal/omniplan"
	"github.com/spf13/cobra"
)

//go:embed templates/__TOC.xml templates/__changelog.xml
var templateFS embed.FS
var epicGroup bool
var milestoneDone bool

var rootCmd = &cobra.Command{
	Use:   "jql-to-plan [project] [JQL]",
	Short: "A CLI to fetch Jira tickets via JQL and output OmniPlan XML",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		jql := args[1]

		cfg, err := config.Load()
		if err != nil {
			if err == config.ErrConfigNotFound {
				fmt.Printf("Configuration file not found.\nPlease run 'jql-to-plan config' to create one.\n")
				os.Exit(1)
			}
			log.Fatalf("Error loading config: %v", err)
		}

		if cfg.JiraURL == "" || cfg.JiraPAT == "" {
			log.Fatal("Error: JIRA_URL and JIRA_PAT must be set via environment variables or config file\nRun 'jql-to-plan config' to edit your configuration.")
		}

		// Enforce optional field for this command
		if cfg.EffortCustomFieldID == "" {
			log.Fatal("Error: effort_custom_field_id is not set in configuration.\nThis field is required for this command.\nPlease run 'jql-to-plan config' and uncomment/set the effort_custom_field_id.")
		}

		if epicGroup && cfg.EpicLinkCustomFieldID == "" {
			log.Fatal("Error: --epic-group flag requires epic_link_custom_field_id to be set in configuration.\nPlease run 'jql-to-plan config' and set the epic_link_custom_field_id.")
		}

		client, err := jira.NewClient(cfg.JiraURL, cfg.JiraPAT, cfg.EffortCustomFieldID, cfg.EpicLinkCustomFieldID)
		if err != nil {
			log.Fatalf("Error creating Jira client: %v", err)
		}

		tickets, epics, err := client.GetTickets(context.Background(), jql)
		if err != nil {
			log.Fatalf("Error fetching tickets: %v", err)
		}

		// Create the .oplx directory
		dirName := projectName + ".oplx"
		if err := os.MkdirAll(dirName, 0755); err != nil {
			log.Fatalf("Error creating directory %s: %v", dirName, err)
		}

		// Write Actual.xml
		actualPath := filepath.Join(dirName, "Actual.xml")
		actualFile, err := os.Create(actualPath)
		if err != nil {
			log.Fatalf("Error creating file %s: %v", actualPath, err)
		}
		defer actualFile.Close()

		serializer := omniplan.NewSerializer(projectName)
		serializer.GroupByEpic = epicGroup
		serializer.MilestoneDone = milestoneDone
		if err := serializer.Serialize(actualFile, tickets, epics); err != nil {
			log.Fatalf("Error serializing to OmniPlan XML: %v", err)
		}

		// Copy __TOC.xml
		if err := copyTemplateFile(templateFS, "templates/__TOC.xml", filepath.Join(dirName, "__TOC.xml")); err != nil {
			log.Fatalf("Error copying __TOC.xml: %v", err)
		}

		// Copy __changelog.xml
		if err := copyTemplateFile(templateFS, "templates/__changelog.xml", filepath.Join(dirName, "__changelog.xml")); err != nil {
			log.Fatalf("Error copying __changelog.xml: %v", err)
		}

		fmt.Printf("Created OmniPlan package: %s\n", dirName)
	},
}

// copyTemplateFile copies a file from the embedded filesystem to the destination path
func copyTemplateFile(fs embed.FS, srcPath, dstPath string) error {
	src, err := fs.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open embedded file %s: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.Flags().BoolVarP(&epicGroup, "epic-group", "e", false, "Group tasks by Epic")
	rootCmd.Flags().BoolVarP(&milestoneDone, "milestone-done", "m", false, "Add a final 'Done' milestone")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
