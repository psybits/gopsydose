package drugdose

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type SourceConfig struct {
	API_ADDRESS string
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
const settingsFilename string = "gpd-settings.toml"

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

// Format the name set by the caller.
func sprintPrefix(name string) string {
	if name != "" {
		return fmt.Sprint(moduleName + ": " + name + ": ")
	}
	return ""
}

// Print the name set by the caller,
// so that it's easier to track the origin of text output.
func printPrefix(name string) {
	if name != "" {
		fmt.Print(sprintPrefix(name))
	}
}

// Print strings properly formatted for the module.
// This is so that when the module is imported, the user can better understand
// where a string is coming from.
// If you only need to add a newline, don't use this function!
func printName(name string, str ...any) {
	printPrefix(name)
	fmt.Println(str...)
}

// Variation of printName(), that doesn't output a newline at the end.
func printNameNoNewline(name string, str ...any) {
	printPrefix(name)
	fmt.Print(str...)
}

// Variation of printName(), that uses fmt.Printf() formatting.
func printNameF(name string, str string, variables ...any) {
	printPrefix(name)
	fmt.Printf(str, variables...)
}

// Same as printName(), but only for verbose output and is optional.
func printNameVerbose(verbose bool, name string, str ...any) {
	if verbose == true {
		printName(name, str...)
	}
}

// Instead of printing, just return the formatted string without a newline.
func sprintName(name string, str ...any) string {
	if name != "" {
		return fmt.Sprintf("%s%s", sprintPrefix(name), fmt.Sprint(str...))
	}
	return ""
}

// Instead of printing, just return the formatted string with a newline.
func sprintlnName(name string, str ...any) string {
	if name != "" {
		return fmt.Sprintf("%s\n", sprintName(name, str...))
	}
	return ""
}

// Initialise the Config struct using the default values.
//
// sourcecfg - The name of the implemented source to use.
// The meaning of "source" is for example an API server for which
// code is present in this repository.
func InitConfigStruct(sourcecfg string) Config {
	cfg := Config{
		MaxLogsPerUser:  DefaultMaxLogsPerUser,
		UseSource:       sourcecfg,
		AutoFetch:       DefaultAutoFetch,
		AutoRemove:      DefaultAutoRemove,
		DBDriver:        DefaultDBDriver,
		VerbosePrinting: DefaultVerbose,
		DBSettings:      nil,
		Timezone:        DefaultTimezone,
	}
	return cfg
}

func (cfg Config) InitSourceMap(apiAddress string) map[string]SourceConfig {
	srcmap := map[string]SourceConfig{
		cfg.UseSource: {
			API_ADDRESS: apiAddress,
		},
	}
	return srcmap
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
func (cfg Config) InitSourceSettings(newcfg map[string]SourceConfig, recreate bool) bool {
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

func (initcfg Config) InitDBSettings(dbdir string, dbname string, mysqlaccess string) Config {
	const printN string = "InitDBSettings()"

	if dbdir == DefaultDBDir {
		home, err := os.UserHomeDir()
		if err != nil {
			printName(printN, err)
			// Return Config struct unchanged, but
			// there needs to be code to handle
			// the unchanged value.
			// Same for all other errors below.
			return initcfg
		}

		path := home + "/.local/share"

		_, err = os.Stat(path)
		if err == nil {
			home = path
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				printName(printN, err)
				return initcfg
			}
		}

		dbdir = home + "/" + dbdir
	}

	var dbSettings = map[string]DBSettings{
		"sqlite3": {
			Path: dbdir + "/" + dbname,
		},
		"mysql": {
			Path: mysqlaccess,
		},
	}

	initcfg.DBSettings = dbSettings

	return initcfg
}

func (initconf Config) InitSettingsFile(recreate bool, verbose bool) bool {
	const printN string = "InitSettingsFile()"

	setdir := InitSettingsDir()
	if setdir == "" {
		return false
	}

	path := setdir + "/" + settingsFilename
	_, err := os.Stat(path)
	if err != nil || recreate {
		if errors.Is(err, os.ErrNotExist) || recreate {
			mcfg, err := toml.Marshal(initconf)
			if err != nil {
				printName(printN, err)
				return false
			}

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
func GetSettings() Config {
	cfg := Config{}

	// Return an empty struct to avoid looking
	// at the wrong path.
	// There needs to be code which can
	// handle the empty struct properly.
	setdir := InitSettingsDir()
	if setdir == "" {
		return cfg
	}

	path := setdir + "/" + settingsFilename

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

func InitAllSettings(sourcecfg string, dbDir string, dbName string, mysqlAccess string,
	recreateSettings bool, recreateSources bool, verbose bool, apiAddress string) Config {
	const printN string = "InitAllSettings()"

	setcfg := InitConfigStruct(sourcecfg)

	setcfg = setcfg.InitDBSettings(dbDir, dbName, mysqlAccess)
	if setcfg.DBSettings == nil {
		printName(printN, "DBSettings not initialised properly.")
		os.Exit(1)
	}

	ret := setcfg.InitSettingsFile(recreateSettings, verbose)
	if !ret {
		printNameVerbose(verbose, "The settings file wasn't initialised.")
	}

	gotsetcfg := GetSettings()
	if len(gotsetcfg.DBDriver) == 0 {
		printName(printN, "Config struct wasn't initialised properly.")
		os.Exit(1)
	}

	if verbose == true {
		gotsetcfg.VerbosePrinting = true
	}

	gotsrc := gotsetcfg.InitSourceMap(apiAddress)

	ret = gotsetcfg.InitSourceSettings(gotsrc, recreateSources)
	if !ret {
		printNameVerbose(gotsetcfg.VerbosePrinting, "The sources file wasn't initialised.")
	}

	return gotsetcfg
}
