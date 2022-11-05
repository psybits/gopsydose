package drugdose

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type SourceConfig struct {
	API_URL string
}

type Config struct {
	MaxLogsPerUser  int16
	UseSource       string
	AutoFetch       bool
	AutoRemove      bool
	DBDriver        string
	VerbosePrinting bool
	DBSettings      map[string]DBSettings
	Timezone        string
}

type DBSettings struct {
	Path string
}

const PsychonautwikiAPI = "api.psychonautwiki.org"

const DefaultMaxLogsPerUser = 100
const DefaultAPI = PsychonautwikiAPI
const DefaultAutoFetch = true
const DefaultDBDir = "GPD"
const DefaultDBName = "gpd.db"
const DefaultAutoRemove = false
const DefaultDBDriver = "sqlite3"
const DefaultMySQLAccess = "user:password@tcp(127.0.0.1:3306)/database"
const DefaultVerbose = false
const DefaultTimezone = "Local"

const DefaultUsername = "defaultUser"
const DefaultSource = "psychonautwiki"

const sourceSetFilename = "gpd-sources.toml"
const setFilename = "gpd-settings.toml"

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
	newcfg := map[string]SourceConfig{
		source: {
			API_URL: api,
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
	configdir = configdir + "/GPD"
	_, err = os.Stat(configdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(configdir, 0700)
			if err != nil {
				fmt.Println("InitSettingsDir:", err)
				return ""
			}
		} else {
			fmt.Println("InitSettingsDir:", err)
			return ""
		}
	}
	return configdir
}

func (cfg Config) InitSourceSettings(newcfg *map[string]SourceConfig, recreate bool) bool {
	mcfg, err := toml.Marshal(newcfg)
	if err != nil {
		fmt.Println(err)
		return false
	}

	setdir := InitSettingsDir()
	if setdir == "" {
		return false
	}

	path := setdir + "/" + sourceSetFilename
	_, err = os.Stat(path)
	if err != nil || recreate {
		if errors.Is(err, os.ErrNotExist) || recreate {
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
		} else {
			otherError(path, err)
		}
	} else if err == nil {
		VerbosePrint("Config file: "+path+" ; already exists!", cfg.VerbosePrinting)
		return false
	}

	return true
}

func GetSourceData() map[string]SourceConfig {
	setdir := InitSettingsDir()
	if setdir == "" {
		return nil
	}

	path := setdir + "/" + sourceSetFilename

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

func InitSettingsStruct(maxulogs int16, source string, autofetch bool,
	dbdir string, dbname string, autoremove bool,
	dbdriver string, mysqlaccess string, verboseprinting bool,
	timezone string) *Config {
	if dbdir == DefaultDBDir {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			return nil
		}

		path := home + "/.local/share"

		_, err = os.Stat(path)
		if err == nil {
			home = path
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				fmt.Println("InitSettingsStruct:", err)
				return nil
			}
		}

		dbdir = home + "/" + dbdir
	}

	var dbSettings = map[string]DBSettings{
		"sqlite3": {
			Path: dbdir + "/" + DefaultDBName,
		},
		"mysql": {
			Path: mysqlaccess,
		},
	}

	initConf := Config{
		MaxLogsPerUser:  maxulogs,
		UseSource:       source,
		AutoFetch:       autofetch,
		AutoRemove:      autoremove,
		DBDriver:        dbdriver,
		VerbosePrinting: verboseprinting,
		DBSettings:      dbSettings,
		Timezone:        timezone,
	}

	return &initConf
}

func (initconf *Config) InitSettings(recreate bool) bool {
	mcfg, err := toml.Marshal(initconf)
	if err != nil {
		fmt.Println(err)
		return false
	}

	setdir := InitSettingsDir()
	if setdir == "" {
		return false
	}

	path := setdir + "/" + setFilename
	_, err = os.Stat(path)
	if err != nil || recreate {
		if errors.Is(err, os.ErrNotExist) || recreate {
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
		} else {
			otherError(path, err)
		}
	} else if err == nil {
		VerbosePrint("Config file: "+path+" ; already exists!", initconf.VerbosePrinting)
		return false
	}

	return true
}

func GetSettings(printfile bool) *Config {
	setdir := InitSettingsDir()
	if setdir == "" {
		return nil
	}

	path := setdir + "/" + setFilename

	cfg := Config{}

	file, err := os.ReadFile(path)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	if printfile {
		fmt.Printf("%s", file)
	}

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	return &cfg
}
