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

type MaxLogsPerUserSize int32

type Config struct {
	MaxLogsPerUser  MaxLogsPerUserSize
	UseSource       string
	AutoFetch       bool
	AutoRemove      bool
	DBDriver        string
	VerbosePrinting bool
	DBSettings      map[string]DBSettings
	Timezone        string
	ProxyURL        string
	Timeout         string
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
const DefaultDBDriver string = SqliteDriver
const DefaultMySQLAccess string = "user:password@tcp(127.0.0.1:3306)/database"
const DefaultVerbose bool = false
const DefaultTimezone string = "Local"
const DefaultProxyURL string = ""
const DefaultTimeout string = "5s"

const DefaultUsername string = "defaultUser"
const DefaultSource string = "psychonautwiki"

const sourceSetFilename string = "gpd-sources.toml"
const settingsFilename string = "gpd-settings.toml"

func errorCantCreateConfig(filename string, err error, printN string) {
	printName(printN, "errorCantCreateConfig(): Error, can't create config file:", filename, ";", err)
	exitProgram(printN)
}

func errorCantCloseConfig(filename string, err error, printN string) {
	printName(printN, "errorCantCloseConfig(): Error, can't close config file:", filename, ";", err)
	exitProgram(printN)
}

func errorCantReadConfig(filename string, err error, printN string) {
	printName(printN, "errorCantReadConfig(): Error, can't read config file:", filename, ";", err)
	exitProgram(printN)
}

func errorCantChmodConfig(filename string, err error, printN string) {
	printName(printN, "errorCantChmodConfig(): Error, can't change mode of config file:", filename, ";", err)
	exitProgram(printN)
}

func otherError(filename string, err error, printN string) {
	printName(printN, "otherError(): Other error for config file:", filename, ";", err)
	exitProgram(printN)
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
func sprintfName(name string, str string, variables ...any) string {
	if name != "" {
		return fmt.Sprintf(sprintPrefix(name)+str, variables...)
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
		ProxyURL:        DefaultProxyURL,
		Timeout:         DefaultTimeout,
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

func InitSettingsDir() (error, string) {
	const printN string = "InitSettingsDir()"

	configdir, err := os.UserConfigDir()
	if err != nil {
		return errors.New(sprintName(printN, err)), ""
	}
	configdir = configdir + "/GPD"
	_, err = os.Stat(configdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(configdir, 0700)
			if err != nil {
				return errors.New(sprintName(printN, err)), ""
			}
		} else {
			return errors.New(sprintName(printN, err)), ""
		}
	}
	return nil, configdir
}

// Create the config file for the sources.
func (cfg Config) InitSourceSettings(newcfg map[string]SourceConfig, recreate bool) bool {
	const printN string = "InitSourceSettings()"

	mcfg, err := toml.Marshal(newcfg)
	if err != nil {
		printName(printN, err)
		return false
	}

	err, setdir := InitSettingsDir()
	if err != nil {
		printName(printN, err)
		return false
	}

	path := setdir + "/" + sourceSetFilename
	_, err = os.Stat(path)
	if err != nil || recreate {
		if errors.Is(err, os.ErrNotExist) || recreate {
			printName(printN, "Initialising config file:", path)
			file, err := os.Create(path)
			if err != nil {
				errorCantCreateConfig(path, err, printN)
			}

			err = file.Chmod(0600)
			if err != nil {
				errorCantChmodConfig(path, err, printN)
			}

			_, err = file.WriteString(string(mcfg))
			if err != nil {
				errorCantCreateConfig(path, err, printN)
			}

			err = file.Close()
			if err != nil {
				errorCantCloseConfig(path, err, printN)
			}
		} else {
			otherError(path, err, printN)
		}
	} else if err == nil {
		printNameVerbose(cfg.VerbosePrinting, printN, "Config file: "+path+" ; already exists!")
		return false
	}

	return true
}

func GetSourceData() map[string]SourceConfig {
	const printN string = "GetSourceData()"

	err, setdir := InitSettingsDir()
	if err != nil {
		printName(printN, err)
		exitProgram(printN)
	}

	path := setdir + "/" + sourceSetFilename

	cfg := map[string]SourceConfig{}

	file, err := os.ReadFile(path)
	if err != nil {
		errorCantReadConfig(path, err, printN)
	}

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err, printN)
	}

	return cfg
}

func (initcfg Config) InitDBSettings(dbdir string, dbname string, mysqlaccess string) (error, Config) {
	const printN string = "InitDBSettings()"

	if dbdir == DefaultDBDir {
		home, err := os.UserHomeDir()
		if err != nil {
			err = errors.New(sprintName(printN, err))
			return err, initcfg
		}

		path := home + "/.local/share"

		_, err = os.Stat(path)
		if err == nil {
			home = path
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				err = errors.New(sprintName(printN, err))
				return err, initcfg
			}
		}

		dbdir = home + "/" + dbdir
	}

	var dbSettings = map[string]DBSettings{
		SqliteDriver: {
			Path: dbdir + "/" + dbname,
		},
		MysqlDriver: {
			Path: mysqlaccess,
		},
	}

	initcfg.DBSettings = dbSettings

	return nil, initcfg
}

func (initconf Config) InitSettingsFile(recreate bool, verbose bool) {
	const printN string = "InitSettingsFile()"

	err, setdir := InitSettingsDir()
	if err != nil {
		printName(printN, err)
		exitProgram(printN)
	}

	path := setdir + "/" + settingsFilename
	_, err = os.Stat(path)
	if err != nil || recreate {
		if errors.Is(err, os.ErrNotExist) || recreate {
			mcfg, err := toml.Marshal(initconf)
			if err != nil {
				printName(printN, err)
				exitProgram(printN)
			}

			printName(printN, "Initialising config file:", path)
			file, err := os.Create(path)
			if err != nil {
				errorCantCreateConfig(path, err, printN)
			}

			err = file.Chmod(0600)
			if err != nil {
				errorCantChmodConfig(path, err, printN)
			}
			_, err = file.WriteString(string(mcfg))
			if err != nil {
				errorCantCreateConfig(path, err, printN)
			}

			err = file.Close()
			if err != nil {
				errorCantCloseConfig(path, err, printN)
			}
		} else {
			otherError(path, err, printN)
		}
	} else {
		printNameVerbose(verbose, printN, "Config file: "+path+" ; already exists!")
	}
}

// Get the settings structure from the general settings config file.
func GetSettings() Config {
	const printN string = "GetSettings()"

	cfg := Config{}

	err, setdir := InitSettingsDir()
	if setdir == "" {
		printName(printN, err)
		exitProgram(printN)
	}

	path := setdir + "/" + settingsFilename

	file, err := os.ReadFile(path)
	if err != nil {
		errorCantReadConfig(path, err, printN)
	}

	err = toml.Unmarshal(file, &cfg)
	if err != nil {
		errorCantReadConfig(path, err, printN)
	}

	return cfg
}

func InitAllSettings(sourcecfg string, dbDir string, dbName string, mysqlAccess string,
	recreateSettings bool, recreateSources bool, verbose bool, apiAddress string) Config {
	const printN string = "InitAllSettings()"

	setcfg := InitConfigStruct(sourcecfg)

	err, setcfg := setcfg.InitDBSettings(dbDir, dbName, mysqlAccess)
	if err != nil {
		printName(printN, err)
		exitProgram(printN)
	}

	setcfg.InitSettingsFile(recreateSettings, verbose)

	gotsetcfg := GetSettings()
	if len(gotsetcfg.DBDriver) == 0 {
		printName(printN, "Config struct wasn't initialised properly.")
		os.Exit(1)
	}

	if verbose == true {
		gotsetcfg.VerbosePrinting = true
	}

	gotsrc := gotsetcfg.InitSourceMap(apiAddress)

	ret := gotsetcfg.InitSourceSettings(gotsrc, recreateSources)
	if !ret {
		printNameVerbose(gotsetcfg.VerbosePrinting, "The sources file wasn't initialised.")
	}

	return gotsetcfg
}
