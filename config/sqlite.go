package config

type SQLite struct {
	FilePath string `yaml:"filePath" json:"filePath" mapstructure:"filePath"`
}
