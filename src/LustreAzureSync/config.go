package main

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	MountRoot     string `yaml:"mount_root"`
	AccountName   string `yaml:"account_name"`
	AccountSuffix string `yaml:"account_suffix"`
	ContainerName string `yaml:"container_name"`
	MdtName       string `yaml:"mdt_name"`
	ArchiveId     string `yaml:"archive_id"`
	ChangelogUser string `yaml:"changelog_user"`
}

func ReadConfigFile(filepath string) (*Config, error) {
	// Read the YAML file
	data, err := os.ReadFile(filepath)
	if err != nil {
		slog.Error("Failed to read config file", "filepath", filepath, "error", err)
		return nil, err
	}

	// Unmarshal the YAML data into a Config struct
	config := &Config{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		slog.Error("Failed to unmarshal config file", "filepath", filepath, "error", err)
		return nil, err
	}

	return config, nil
}
