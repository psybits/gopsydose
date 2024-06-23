package drugdose

import (
	"errors"
	"fmt"
	"os"

	cp "github.com/otiai10/copy"

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
	CostCurrency    string
}

type DBSettings struct {
	Path       string
	Parameters string
}

const PsychonautwikiAddress string = "api.psychonautwiki.org"

const DefaultMaxLogsPerUser MaxLogsPerUserSize = 100
const DefaultSourceAddress string = PsychonautwikiAddress
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
const DefaultCostCurr string = ""

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
		CostCurrency:    DefaultCostCurr,
	}
	return cfg
}

// InitSourceMap returns a map which for the given key (configured source)
// returns the address as it's value. The address could be an IP address,
// an URL and etc.
//
// apiAddress - the address to map to the source name from the Config struct
func (cfg *Config) InitSourceMap(apiAddress string) map[string]SourceConfig {
	srcmap := map[string]SourceConfig{
		cfg.UseSource: {
			API_ADDRESS: apiAddress,
		},
	}
	return srcmap
}

// InitSettingsDir creates the directory for the configuration files using the
// system path for configs and sets the proper mode to the new directory.
// It first checks if it already exists, skips the creation if true.
//
// Returns the full path to the directory as a string.
func InitSettingsDir() (error, string) {
	const printN string = "InitSettingsDir()"

	configdir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err), ""
	}
	configdir = configdir + "/GPD"
	_, err = os.Stat(configdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(configdir, 0700)
			if err != nil {
				return fmt.Errorf("%s%w", sprintName(printN), err), ""
			}
		} else {
			return fmt.Errorf("%s%w", sprintName(printN), err), ""
		}
	}
	return nil, configdir
}

// InitSourceSettings creates the config file for the sources. This file
// contains the api name mapped to the api address. InitSourceMap() can be used
// to create the map, this function marshals it and writes it to the actual
// config file.
//
// newcfg - the source api name to api address map
//
// recreate - overwrite the current file if it exists with a new map
func (cfg *Config) InitSourceSettings(newcfg map[string]SourceConfig, recreate bool) error {
	const printN string = "InitSourceSettings()"

	mcfg, err := toml.Marshal(newcfg)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
	}

	err, setdir := InitSettingsDir()
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
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
		return nil
	}

	return nil
}

// GetSourceData returns the map gotten from the source config file
// unmarshaled. The map returns for a given source name (key), an addres
// (value) for that source.
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

// InitDBSettings returns a modified Config structure, which contains a properly
// formatted DBSettings map. Before using the modified struct, checkout if
// the returned error is not nil!
//
// dbdir - if this is set to the DefaultDBDir constant, it will try to use
// the system user directory as a path, if not the full path must be specified
//
// dbname - the name of sqlite db file
//
// mysqlaccess - the path for connecting to MySQL/MariaDB, example
// user:password@tcp(127.0.0.1:3306)/database
func (initcfg *Config) InitDBSettings(dbdir string, dbname string, mysqlaccess string) (error) {
	const printN string = "InitDBSettings()"

	if dbdir == DefaultDBDir {
		home, err := os.UserHomeDir()
		if err != nil {
			err = fmt.Errorf("%s%w", sprintName(printN), err)
			return err
		}

		path := home + "/.local/share"

		_, err = os.Stat(path)
		if err == nil {
			home = path
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				err = fmt.Errorf("%s%w", sprintName(printN), err)
				return err
			}
		}

		dbdir = home + "/" + dbdir
	}

	var dbSettings = map[string]DBSettings{
		SqliteDriver: {
			Path:       dbdir + "/" + dbname,
			Parameters: "?_pragma=busy_timeout=1000",
		},
		MysqlDriver: {
			Path:       mysqlaccess,
			Parameters: "",
		},
	}

	initcfg.DBSettings = dbSettings

	return nil
}

// InitSettingsFile creates and fills the main global config file which
// is used for the Config struct. It sets the proper mode and stops the
// program on error. The data for writing to the file is taken from the passed
// Config structure.
//
// recreate - if true overwrites the currently existing config file with the
// currently passed Config struct data
//
// verbose - whether to print verbose information
func InitSettingsFile(recreate bool, verbose bool, sourcecfg string, dbDir string, dbName string, mysqlAccess string) {
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
			initconf := InitConfigStruct(sourcecfg)

			err := initconf.InitDBSettings(dbDir, dbName, mysqlAccess)
			if err != nil {
				printName(printN, err)
				exitProgram(printN)
			}

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

// InitNamesFiles copies to the OS config directory, the directory containing
// the toml files for configuring alternative names. If it doesn't exists in
// the config directory, the code checks if it's present in the current working
// directory. If it is, it's copied over to the OS config directory and used
// later to fill in the database.
func (cfg *Config) InitNamesFiles() error {
	const printN string = "InitNamesFiles()"

	err, setdir := InitSettingsDir()
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
	}

	var CopyToPath string = setdir + "/" + allNamesConfigsDir

	// Check if names directory exists in config directory.
	// If it doen't, continue.
	_, err = os.Stat(CopyToPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Check if names directory exists in working directory.
			// If it does, copy it to config directory.
			_, err := os.Stat(allNamesConfigsDir)
			if err == nil {
				printName(printN, "Found the config directory in the working directory:",
					allNamesConfigsDir, "; attempt at making a copy to:", CopyToPath)

				// Sync (true) - flush everything to disk, to make sure everything is immediately copied
				cpOpt := cp.Options{
					Sync: true,
				}
				err = cp.Copy(allNamesConfigsDir, CopyToPath, cpOpt)
				if err != nil {
					return fmt.Errorf("%s%w", sprintName(printN), err)
				} else if err == nil {
					printName(printN, "Done copying to:", CopyToPath)
				}
			} else {
				return fmt.Errorf("%s%w", sprintName(printN), err)
			}
		} else {
			return fmt.Errorf("%s%w", sprintName(printN), err)
		}
	} else if err == nil {
		printNameVerbose(cfg.VerbosePrinting, printN, "Name config already exists:", CopyToPath,
			"; will not copy the config directory from the working directory:", allNamesConfigsDir)
	}

	return nil
}

// Get the Config structure data marshaled from the global config file.
// Returns the Config structure. Stops the program if an error is not nil.
func GetSettings() Config {
	const printN string = "GetSettings()"

	cfg := Config{}

	err, setdir := InitSettingsDir()
	if err != nil {
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

// InitAllSettings initializes the Config struct with default values,
// uses the struct to create the global config file, unmarshales the newly
// created config and stores the struct in a new variable, initializes the
// source map using the set source in the Config struct and the given API
// address, uses the map to create the source config file and returns the
// Config struct. It then copies the directory containing the toml files for
// filling alternative names to the database.
//
// Basically all you probably would want to do anyway, but in a single function.
//
// Checkout InitConfigStruct(), InitDBSettings(), InitSettingsFile()
// GetSettings(), InitSourceMap(), InitSourceSettings() for more info.
//
// sourcecfg - what source name to set by default for the Config struct
//
// dbDir - the path for using the sqlite database file
//
// dbName - the name of the sqlite db file
//
// mysqlAccess - the path for accessing an MySQL/MariaDB database
//
// recreateSettings - overwrite the settings file even if it exists
//
// recreateSources - overwrite the source settings file even if it exists
//
// verbose - if true print verbose information
//
// apiAddress - the address to use when initializing the source map
func InitAllSettings(sourcecfg string, dbDir string, dbName string, mysqlAccess string,
	recreateSettings bool, recreateSources bool, verbose bool, apiAddress string) Config {
	const printN string = "InitAllSettings()"

	InitSettingsFile(recreateSettings, verbose, sourcecfg, dbDir, dbName, mysqlAccess)

	gotsetcfg := GetSettings()

	if verbose == true {
		gotsetcfg.VerbosePrinting = true
	}

	gotsrc := gotsetcfg.InitSourceMap(apiAddress)

	err := gotsetcfg.InitSourceSettings(gotsrc, recreateSources)
	if err != nil {
		printName(printN, "The sources file wasn't initialised: ", err)
		exitProgram(printN)
	}

	err = gotsetcfg.InitNamesFiles()
	if err != nil {
		printName(printN, "The names files were not copied: ", err)
		exitProgram(printN)
	}

	return gotsetcfg
}
