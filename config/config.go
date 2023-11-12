package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
}

type ViperUnmarshaler interface {
	Unmarshal(interface{}, ...viper.DecoderConfigOption) error
}

func Init(file string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(file)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	viper.SetEnvPrefix("FEEDREADER")
	viper.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", ""))

	c := new(Config)
	err := v.Unmarshal(c)
	return c, err
}
