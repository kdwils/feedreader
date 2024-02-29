package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	SQLite SQLite `mapstructure:"sqlite"`
	Port   int    `mapstructure:"port"`
}

func Init(file string) (*Config, error) {
	v := viper.New()
	v.AddConfigPath(".")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	err := v.ReadInConfig()
	if err != nil {
		return nil, err
	}

	v.SetEnvPrefix("FEEDREADER")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", ""))

	c := new(Config)
	err = v.Unmarshal(c)
	return c, err
}
