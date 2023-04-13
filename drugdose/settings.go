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

type MaxLogsPerUserSize int16

type Config struct {
	MaxLogsPerUser  MaxLogsPerUserSize
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

const PsychonautwikiAPI string = "api.psychonautwiki.org"

const DefaultMaxLogsPerUser MaxLogsPerUserSize = 100
const DefaultAPI string = PsychonautwikiAPI
const DefaultAutoFetch bool = true
const DefaultDBDir string = "GPD"
const DefaultDBName string = "gpd.db"
const DefaultAutoRemove bool = false
const DefaultDBDriver string = "sqlite3"
const DefaultMySQLAccess string = "user:password@tcp(127.0.0.1:3306)/database"
const DefaultVerbose bool = false
const DefaultTimezone string = "Local"

const DefaultUsername string = "defaultUser"
const DefaultSource string = "psychonautwiki"

const sourceSetFilename string = "gpd-sources.toml"
const setFilename string = "gpd-settings.toml"

func errorCantCreateConfig(filename string, err error) {
	printName("errorCantCreateConfig()", "Error, can't create config file:", filename, ";", err)
	exitProgram()
}

func errorCantCloseConfig(filename string, err error) {
	printName("errorCantCloseConfig()", "Error, can't close config file:", filename, ";", err)
	exitProgram()
}

func errorCantReadConfig(filename string, err error) {
	printName("errorCantReadConfig()", "Error, can't read config file:", filename, ";", err)
	exitProgram()
}

func errorCantChmodConfig(filename string, err error) {
	printName("errorCantChmodConfig()", "Error, can't change mode of config file:", filename, ";", err)
	exitProgram()
}

func otherError(filename string, err error) {
	printName("otherError()", "Other error for config file:", filename, ";", err)
	exitProgram()
}

// The name used when printing, to distinguish from other logs.
const moduleName string = "gopsydose"

// Print strings properly formatted for the module.
// This is so that when the module is imported, the user can better understand
// where a string is coming from.
// If you only need to add a newline, don't use this function!
func printName(name string, str ...any) {
	fmt.Print(moduleName + ": " + name + ": ")
	fmt.Println(str...)
}

// Variation of printName(), that doesn't output a newline at the end.
func printNameNoNewline(name string, str ...any) {
	fmt.Print(moduleName + ": " + name + ": ")
	fmt.Print(str...)
}

// Variation of printName(), that uses fmt.Printf() formatting.
func printNameF(name string, str string, variables ...any) {
	fmt.Print(moduleName + ": " + name + ": ")
	fmt.Printf(str, variables...)
}

// Same as printName(), but only for verbose output and is optional.
func printNameVerbose(verbose bool, name string, str ...any) {
	if verbose == true {
		printName(name, str...)
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
	const printN string = "InitSettingsDir()"

	configdir, err := os.UserConfigDir()
	if err != nil {
		printName(printN, err)
		return ""
	}
	configdir = configdir + "/GPD"
	_, err = os.Stat(configdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(configdir, 0700)
			if err != nil {
				printName(printN, err)
				return ""
			}
		} else {
			printName(printN, err)
			return ""
		}
	}
	return configdir
}

// Create the config file for the sources.
func (cfg Config) InitSourceSettings(newcfg *map[string]SourceConfig, recreate bool) bool {
	const printN string = "InitSourceSettings()"

	mcfg, err := toml.Marshal(newcfg)
	if err != nil {
		printName(printN, err)
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
			printName(printN, "Initialising config file:", path)
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
		printNameVerbose(cfg.VerbosePrinting, printN, "Config file: "+path+" ; already exists!")
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

func InitSettingsStruct(maxulogs MaxLogsPerUserSize, source string, autofetch bool,
	dbdir string, dbname string, autoremove bool,
	dbdriver string, mysqlaccess string, verboseprinting bool,
	timezone string) *Config {

	const printN string = "InitSettingsStruct()"

	if dbdir == DefaultDBDir {
		home, err := os.UserHomeDir()
		if err != nil {
			printName(printN, err)
			return nil
		}

		path := home + "/.local/share"

		_, err = os.Stat(path)
		if err == nil {
			home = path
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				printName(printN, err)
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

func (initconf *Config) InitSettings(recreate bool, verbose bool) bool {
	const printN string = "InitSettings()"

	mcfg, err := toml.Marshal(initconf)
	if err != nil {
		printName(printN, err)
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
			printName(printN, "Initialising config file:", path)
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
	} else {
		printNameVerbose(verbose, printN, "Config file: "+path+" ; already exists!")
		return false
	}

	return true
}

// Get the settings structure from the general settings config file.
func GetSettings() *Config {
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

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err)
	}

	return &cfg
}
