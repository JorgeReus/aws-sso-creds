package config

import (
	"fmt"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

const configName = "aws-sso-creds"

type Config struct {
	Orgs             map[string]Organization `mapstructure:"organizations"     validate:"required"`
	Home             string                  `                                 validate:"required"`
	ErrorColor       string                  `mapstructure:"error_color"       validate:"required"`
	InformationColor string                  `mapstructure:"information_color" validate:"required"`
	WarningColor     string                  `mapstructure:"warning_color"     validate:"required"`
	FocusColor       string                  `mapstructure:"focus_color"       validate:"required"`
	SpinnerColor     string                  `mapstructure:"spinner_color"     validate:"required"`
}

var c *Config
var lock = &sync.Mutex{}

type Organization struct {
	Name   string `validate:"required"`
	Prefix string `validate:"required" mapstructure:"prefix"`
	URL    string `validate:"required" mapstructure:"url"`
	Region string `validate:"required" mapstructure:"region"`
}

func setDefaults() {
	viper.SetDefault("error_color", "#fa0718")
	viper.SetDefault("information_color", "#05fa5f")
	viper.SetDefault("warning_color", "#f29830")
	viper.SetDefault("focus_color", "#4287f5")
	viper.SetDefault("spinner_color", "#42f551")
}

func GetInstance() *Config {
	return c
}

func init() {
	setDefaults()
}

func Init(home string, configPath string) error {
	if c == nil {
		lock.Lock()
		aux := Config{}
		defer lock.Unlock()
		aux.Home = home
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				return fmt.Errorf("Error reading config file: %s", err)
			}
		}

		if err := viper.Unmarshal(&aux); err != nil {
			return fmt.Errorf("unable to unmarshall the config %v", err)
		}

		// Workaround for using the map key as the name
		for k, v := range aux.Orgs {
			v.Name = k
			aux.Orgs[k] = v
		}

		validate := validator.New()
		if err := validate.Struct(&aux); err != nil {
			return fmt.Errorf("Missing required attributes %v\n", err)
		}
		c = &aux
	}

	return nil
}
