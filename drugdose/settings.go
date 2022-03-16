package drugdose

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type SourceConfig struct {
	ApiUrl string
}

type Config struct {
	MaxLogsPerUser int16
	UseAPI         string
	AutoFetch      bool
	DBDir          string
	AutoRemove     bool
}

const psychonautwiki_api = "api.psychonautwiki.org"

const default_api = psychonautwiki_api

const source_set_filename = "gpd-sources.toml"
const set_filename = "gpd-settings.toml"

func errorCantCreateConfig(filename string, err error) {
	fmt.Println("Error, can't create config file:", filename, ";", err)
	exitProgram()
}

func errorCantCloseConfig(filename string, err error) {
	fmt.Println("Error, can't close config file:", filename, ";", err)
	exitProgram()
}

func errorCantReadConfig(filename string, err error) {
	fmt.Println("Error, can't read config file:", filename, ";", err)
	exitProgram()
}

func errorCantChmodConfig(filename string, err error) {
	fmt.Println("Error, can't change mode of config file:", filename, ";", err)
	exitProgram()
}

func otherError(filename string, err error) {
	fmt.Println("Other error for config file:", filename, ";", err)
	exitProgram()
}

func VerbosePrint(prstr string, verbose bool) {
	if verbose {
		fmt.Println(prstr)
	}
}

func InitSourceStruct(source string, api string) *map[string]SourceConfig {
	if source == "default" {
		source = default_source
	}

	if api == "default" {
		api = default_api
	}

	newcfg := map[string]SourceConfig{
		source: {
			ApiUrl: api,
		},
	}
	return &newcfg
}

func InitSettingsDir() string {
	configdir, err := os.UserConfigDir()
	if err != nil {
		fmt.Println(err)
		return ""
	}
	configdir = configdir + "/gpdset"
	if _, err := os.Stat(configdir); errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(configdir, 0700)
		if err != nil {
			fmt.Println(err)
			return ""
		}
	}
	return configdir
}

func InitSourceSettings(newcfg *map[string]SourceConfig, recreate bool, verbose bool) bool {
	mcfg, err := toml.Marshal(newcfg)
	if err != nil {
		fmt.Println(err)
		return false
	}

	setdir := InitSettingsDir()
	if setdir == "" {
		return false
	}

	path := setdir + "/" + source_set_filename
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) || recreate {
		fmt.Println("Initialising config file:", path)
		file, err := os.Create(path)
		if err != nil {
			errorCantCreateConfig(path, err)
		}

		err = file.Chmod(0600)
		if err != nil {
			errorCantChmodConfig(path, err)
		}

		_, err = file.WriteString(string(mcfg))
		if err != nil {
			errorCantCreateConfig(path, err)
		}

		err = file.Close()
		if err != nil {
			errorCantCloseConfig(path, err)
		}
	} else if err == nil {
		VerbosePrint("Config file: "+path+" ; already exists!", verbose)
		return false
	} else {
		otherError(path, err)
	}
	return true
}

func GetSourceData() map[string]SourceConfig {
	setdir := InitSettingsDir()
	if setdir == "" {
		return nil
	}

	path := setdir + "/" + source_set_filename

	cfg := map[string]SourceConfig{}

	file, err := os.ReadFile(path)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	return cfg
}

func InitSettingsStruct(maxulogs int16, source string,
	autofetch bool, dbdir string, autoremove bool) *Config {
	if source == "default" {
		source = default_source
	}

	if dbdir == "default" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			return nil
		}
		dbdir = home + "/" + db_dir
	}

	initConf := Config{
		MaxLogsPerUser: maxulogs,
		UseAPI:         source,
		AutoFetch:      autofetch,
		DBDir:          dbdir,
		AutoRemove:     autoremove,
	}

	return &initConf
}

func (initconf *Config) InitSettings(recreate bool, verbose bool) bool {
	mcfg, err := toml.Marshal(initconf)
	if err != nil {
		fmt.Println(err)
		return false
	}

	setdir := InitSettingsDir()
	if setdir == "" {
		return false
	}

	path := setdir + "/" + set_filename
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) || recreate {
		fmt.Println("Initialising config file:", path)
		file, err := os.Create(path)
		if err != nil {
			errorCantCreateConfig(path, err)
		}

		err = file.Chmod(0600)
		if err != nil {
			errorCantChmodConfig(path, err)
		}

		_, err = file.WriteString(string(mcfg))
		if err != nil {
			errorCantCreateConfig(path, err)
		}

		err = file.Close()
		if err != nil {
			errorCantCloseConfig(path, err)
		}
	} else if err == nil {
		VerbosePrint("Config file: "+path+" ; already exists!", verbose)
		return false
	} else {
		otherError(path, err)
	}
	return true
}

func GetSettings() *Config {
	setdir := InitSettingsDir()
	if setdir == "" {
		return nil
	}

	path := setdir + "/" + set_filename

	cfg := Config{}

	file, err := os.ReadFile(path)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	return &cfg
}
