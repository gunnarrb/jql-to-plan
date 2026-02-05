package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var ErrConfigNotFound = errors.New("configuration file not found")

type Config struct {
	JiraURL               string `mapstructure:"jira_url"`
	JiraPAT               string `mapstructure:"jira_pat"`
	EffortCustomFieldID   string `mapstructure:"effort_custom_field_id"`
	EpicLinkCustomFieldID string `mapstructure:"epic_link_custom_field_id"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Environment variables
	v.SetEnvPrefix("APP") // e.g. APP_JIRA_URL
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Also look for specific JIRA_ env vars as requested
	v.BindEnv("jira_url", "JIRA_URL")
	v.BindEnv("jira_pat", "JIRA_PAT")

	// Config file
	v.SetConfigName(".jql-to-plan")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Check if essential env vars are present, if so, we can proceed without a config file
			if os.Getenv("JIRA_URL") != "" && os.Getenv("JIRA_PAT") != "" {
				// Proceed with env vars
			} else {
				return nil, ErrConfigNotFound
			}
		} else {
			return nil, err
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}

	// Basic validation (though caller might do more specific checks)
	if c.JiraURL == "" || c.JiraPAT == "" {
		// Just a warning or error? For now let's just log, caller might validate
		log.Println("Warning: JIRA_URL or JIRA_PAT not found in config or environment")
	}

	return &c, nil
}

// GetConfigPath returns the path to the config file, or where it should be.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find user home directory: %w", err)
	}
	return filepath.Join(home, ".jql-to-plan.yaml"), nil
}
