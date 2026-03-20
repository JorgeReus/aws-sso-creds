package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/pelletier/go-toml"
	"github.com/spf13/viper"
)

const configName = "aws-sso-creds"

type Config struct {
	Orgs             map[string]Organization `mapstructure:"organizations"     validate:"required,dive"`
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
	Name          string `validate:"required"`
	Prefix        string `validate:"required" mapstructure:"prefix"`
	URL           string `validate:"required" mapstructure:"url"`
	Region        string `mapstructure:"region"`
	SSORegion     string `mapstructure:"sso_region"`
	DefaultRegion string `mapstructure:"default_region"`
}

func (o Organization) EffectiveSSORegion() string {
	if o.SSORegion != "" {
		return o.SSORegion
	}
	return o.Region
}

func (o Organization) EffectiveDefaultRegion() string {
	if o.DefaultRegion != "" {
		return o.DefaultRegion
	}
	return o.EffectiveSSORegion()
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

func ResetForTest() {
	lock.Lock()
	defer lock.Unlock()

	c = nil
	viper.Reset()
	setDefaults()
}

func SetInstanceForTest(cfg *Config) {
	lock.Lock()
	defer lock.Unlock()

	c = cfg
}

func ResetAndSetTestConfig(home string) error {
	ResetForTest()
	SetInstanceForTest(&Config{
		Home:             home,
		Orgs:             map[string]Organization{},
		ErrorColor:       "#fa0718",
		InformationColor: "#05fa5f",
		WarningColor:     "#f29830",
		FocusColor:       "#4287f5",
		SpinnerColor:     "#42f551",
	})
	return nil
}

func UpsertOrganizationConfig(configPath string, org Organization) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	tree, err := toml.Load("")
	if err != nil {
		return err
	}
	if _, err := os.Stat(configPath); err == nil {
		loaded, err := toml.LoadFile(configPath)
		if err != nil {
			return err
		}
		tree = loaded
	}

	defaults := map[string]string{
		"error_color":       "#fa0718",
		"information_color": "#05fa5f",
		"warning_color":     "#f29830",
		"focus_color":       "#4287f5",
		"spinner_color":     "#42f551",
	}
	for key, value := range defaults {
		if !tree.Has(key) {
			tree.Set(key, value)
		}
	}

	if raw := tree.Get("organizations"); raw != nil {
		if orgs, ok := raw.(*toml.Tree); ok {
			for _, existingName := range orgs.Keys() {
				if existingName == org.Name {
					continue
				}
				existingURL := tree.Get(fmt.Sprintf("organizations.%s.url", existingName))
				if existingURL == org.URL {
					return fmt.Errorf("organization %q already uses start URL %q", existingName, org.URL)
				}
				existingPrefix := tree.Get(fmt.Sprintf("organizations.%s.prefix", existingName))
				if existingPrefix == org.Prefix {
					return fmt.Errorf("organization %q already uses prefix %q", existingName, org.Prefix)
				}
			}
		}
	}

	orgPath := fmt.Sprintf("organizations.%s", org.Name)
	tree.Set(fmt.Sprintf("%s.url", orgPath), org.URL)
	tree.Set(fmt.Sprintf("%s.prefix", orgPath), org.Prefix)
	tree.Delete(fmt.Sprintf("%s.region", orgPath))
	tree.Delete(fmt.Sprintf("%s.sso_region", orgPath))
	tree.Delete(fmt.Sprintf("%s.default_region", orgPath))
	tree.Set(fmt.Sprintf("%s.sso_region", orgPath), org.EffectiveSSORegion())
	if org.DefaultRegion != "" && org.DefaultRegion != org.EffectiveSSORegion() {
		tree.Set(fmt.Sprintf("%s.default_region", orgPath), org.DefaultRegion)
	}

	return os.WriteFile(configPath, []byte(tree.String()), 0o644)
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
		for name, org := range aux.Orgs {
			if org.EffectiveSSORegion() == "" {
				return fmt.Errorf("Missing required attributes organizations.%s.sso_region\n", name)
			}
		}
		c = &aux
	}

	return nil
}
