package drugdose

import (
	"errors"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/powerjungle/goalconvert/alconvert"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"

	cp "github.com/otiai10/copy"
)

type AlternativeNames struct {
	AltNames []string
}

type SubstanceName struct {
	LocalName map[string]AlternativeNames
}

const namesSubstanceFilename string = "gpd-substance-names.toml"
const namesRouteFilename string = "gpd-route-names.toml"
const namesUnitsFilename string = "gpd-units-names.toml"
const namesConvUnitsFilename string = "gpd-units-conversions.toml"

const sourceNamesDir string = "source-names-local-configs"

const allNamesConfigsDir string = "gpd-names-configs"

const namesMagicWord string = "!TheTableIsNotEmpty!"

// Read the config file for matching names and return the proper struct.
// nameType - checkout namesFiles()
// source - if not empty, will read the source specific config
func GetNamesConfig(nameType string, source string) *SubstanceName {
	const printN string = "GetNamesConfig()"

	setdir := InitSettingsDir()
	if setdir == "" {
		return nil
	}

	gotFile := namesFiles(nameType)
	if gotFile == "" {
		return nil
	}

	if source != "" {
		gotFile = allNamesConfigsDir + "/" + sourceNamesDir + "/" + source + "/" + gotFile
	} else {
		gotFile = allNamesConfigsDir + "/" + gotFile
	}

	path := setdir + "/" + gotFile

	subName := SubstanceName{}

	file, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			printName(printN, "Error:", err)
		}
		return nil
	}

	err = toml.Unmarshal(file, &subName)
	if err != nil {
		printName(printN, "Unmarshal error:", err)
		return nil
	}

	return &subName
}

func namesTables(nameType string) string {
	const printN string = "namesTables()"

	table := ""
	if nameType == "substance" {
		table = altNamesSubsTableName
	} else if nameType == "route" {
		table = altNamesRouteTableName
	} else if nameType == "units" {
		table = altNamesUnitsTableName
	} else if nameType == "convUnits" {
		table = altNamesConvUnitsTableName
	} else {
		printName(printN, "No nameType:", nameType)
	}

	return table
}

func namesFiles(nameType string) string {
	const printN string = "namesFiles()"

	file := ""
	if nameType == "substance" {
		file = namesSubstanceFilename
	} else if nameType == "route" {
		file = namesRouteFilename
	} else if nameType == "units" {
		file = namesUnitsFilename
	} else if nameType == "convUnits" {
		file = namesConvUnitsFilename
	} else {
		printName(printN, "No nameType:", nameType)
	}

	return file
}

// Copy the config files to the proper directory and read them to
// create the proper tables in the database, which will later be used
// to match alternative names to local names.
// nameType - checkout namesTables() and namesFiles()
// sourceNames - if true, will add data to the source specific config tables
// overwrite - force overwrite of directory and tables
func (cfg Config) AddToSubstanceNamesTable(nameType string, sourceNames bool, overwrite bool) bool {
	const printN string = "AddToSubstanceNamesTable()"

	var setdir string = InitSettingsDir()
	if setdir == "" {
		printName(printN, "No settings directory found!")
		return false
	}

	var CopyToPath string = setdir + "/" + allNamesConfigsDir

	if overwrite == true {
		ret := cfg.CleanNames()
		if ret == false {
			printName(printN, "Couldn't clean names from database for overwrite.")
			return false
		}

		ret = cfg.InitAllDBTables()
		if !ret {
			printName(printN, "Database didn't get initialised, because of an error.")
			return false
		}

		err := os.RemoveAll(CopyToPath)
		if err != nil {
			printName(printN, err)
			return false
		} else {
			printName(printN, "Deleted directory:", CopyToPath)
		}
	}

	table := namesTables(nameType)
	if table == "" {
		return false
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	ret := checkIfExistsDB("localName",
		table,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		namesMagicWord)
	if ret {
		return true
	}

	// Check if names directory exists in config directory.
	// If it doen't, continue.
	_, err := os.Stat(CopyToPath)
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
					printName(printN, "Couldn't move file:", err)
					return false
				} else if err == nil {
					printName(printN, "Done copying to:", CopyToPath)
				}
			} else {
				printName(printN, err)
				return false
			}
		} else {
			printName(printN, err)
			return false
		}
	} else if err == nil {
		printNameVerbose(cfg.VerbosePrinting, printN, "Name config already exists:", CopyToPath,
			"; will not copy the config directory from the working directory:", allNamesConfigsDir)
	}

	getCfgSrc := ""
	if sourceNames {
		getCfgSrc = cfg.UseSource
	}

	namesCfg := GetNamesConfig(nameType, getCfgSrc)
	if namesCfg == nil {
		return false
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	subsStmt, err := db.Prepare("insert into '" + table +
		"' (localName, alternativeName) " +
		"values(?, ?)")
	if err != nil {
		printName(printN, err)
		return false
	}
	defer subsStmt.Close()

	tx, err := db.Begin()
	if err != nil {
		printName(printN, err)
		return false
	}

	_, err = tx.Stmt(subsStmt).Exec(namesMagicWord, namesMagicWord)
	if err != nil {
		printName(printN, err)
		return false
	}

	for locName, altNames := range namesCfg.LocalName {
		locName = strings.ReplaceAll(locName, "_", " ")
		altName := altNames.AltNames
		for i := 0; i < len(altName); i++ {
			_, err = tx.Stmt(subsStmt).Exec(locName, altName[i])
			if err != nil {
				printName(printN, err)
				return false
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		printName(printN, err)
		return false
	}

	printName(printN, nameType, "names initialized successfully! sourceNames:", sourceNames)

	return true
}

// Returns the local name for a given alternative name.
// inputName - the alternative name
// nameType - checkout namesTables()
// sourceNames - if true, it will use the config for the source,
// meaning the names specific for the source
// overwrite - if true will overwrite the names config directory and tables with the currently present ones
func (cfg Config) MatchName(inputName string, nameType string, sourceNames bool, overwrite bool) string {
	const printN string = "MatchName()"

	table := namesTables(nameType)
	if table == "" {
		return inputName
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	ret := cfg.AddToSubstanceNamesTable(nameType, sourceNames, overwrite)
	if !ret {
		return inputName
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	checkCol := []string{"localName", "alternativeName"}
	var gotDBName string
	for i := 0; i < len(checkCol); i++ {
		gotDBName = ""
		err = db.QueryRow("select localName from '"+table+
			"' where "+checkCol[i]+" = ?", inputName).Scan(&gotDBName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) == false {
				printName(printN, "For input:", inputName, "; Error:", err)
				return inputName
			}
		}

		if gotDBName != "" {
			return gotDBName
		}
	}

	return inputName
}

// Returns the local name, using both the global config and the source specific config.
func (cfg Config) MatchAndReplace(inputName string, nameType string, overwrite bool) string {
	ret := cfg.MatchName(inputName, nameType, false, overwrite)
	ret = cfg.MatchName(ret, nameType, true, false)
	return ret
}

// Returns all alternative names for a given local name.
func (cfg Config) GetAllNames(inputName string, nameType string, sourceNames bool) []string {
	const printN string = "GetAllNames()"

	table := namesTables(nameType)
	if table == "" {
		return nil
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	cfg.AddToSubstanceNamesTable(nameType, sourceNames, false)

	repName := cfg.MatchName(inputName, nameType, true, false)
	if repName != inputName {
		printNameVerbose(cfg.VerbosePrinting, printN, "For source:", cfg.UseSource,
			"; Local name:", inputName, "; Is sourceNamesd with:", repName)
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var allNames []string
	var tempName string
	rows, err := db.Query("select alternativeName from '"+table+
		"' where localName = ?", repName)
	if err != nil {
		printName(printN, "Error:", err)
		return nil
	}

	for rows.Next() {
		err = rows.Scan(&tempName)
		if err != nil {
			printName(printN, "Scan: Error:", err)
			return nil
		}
		allNames = append(allNames, tempName)
	}

	addToErrMsg := ""
	if sourceNames {
		addToErrMsg = "; for source: " + cfg.UseSource
	}

	if len(allNames) == 0 {
		printName(printN, "No names found for:", repName, addToErrMsg)
	}

	return allNames
}

// Converts percentage to pure substance.
// input 0 - the total dose
// input 1 - the percentage
// output - pure substance calculated using the percentage
func convPerc2Pure(substance string, unitInputs ...float32) float32 {
	av := alconvert.NewAV()
	av.UserSet.Milliliters = unitInputs[0]
	av.UserSet.Percent = unitInputs[1]
	av.CalcPureAmount()
	return av.GotPure()
}

// Converts pure amount to grams.
// input 0 - the total dose
// input 1 - the percentage
// output - pure substance ml converted to grams using a constant density
func convMl2Grams(substance string, unitInputs ...float32) float32 {
	const printN string = "convMl2Grams()"

	var multiplier float32 = 0

	// g/sm3
	substancesDensities := map[string]float32{
		"Alcohol": 0.79283, // At 16 C temperature
	}

	multiplier = substancesDensities[substance]

	if multiplier == 0 {
		printName(printN, "No density for substance:", substance)
	}

	return convPerc2Pure(substance, unitInputs...) * multiplier
}

type convF func(string, ...float32) float32

func addConversion(cF convF, output float32, name string, inputName string, inputsAmount int, substance string, unitInputs ...float32) float32 {
	if output != 0 {
		return output
	}

	const printN string = "addConversion()"

	if inputName == name {
		gotLenOfUnitInputs := len(unitInputs)
		if gotLenOfUnitInputs == inputsAmount {
			output = cF(substance, unitInputs...)
		} else {
			printName(printN, "Wrong amount of unitInputs:",
				gotLenOfUnitInputs, "; needed", inputsAmount)
		}
	}

	return output
}

func unitsFunctionsOutput(inputName string, substance string, unitInputs ...float32) float32 {
	output := addConversion(convPerc2Pure, 0, "Convert-Percent-To-Pure", inputName, 2, substance, unitInputs...)
	output = addConversion(convMl2Grams, output, "Convert-Milliliters-To-Grams", inputName, 2, substance, unitInputs...)

	return output
}

func (cfg Config) ConvertUnits(substance string, unitInputs ...float32) (float32, string) {
	const printN string = "ConvertUnits()"

	allNames := cfg.GetAllNames(substance, "convUnits", true)
	gotAllNamesLen := len(allNames)
	if gotAllNamesLen != 2 {
		printName(printN, "Wrong amount of names:", gotAllNamesLen,
			"; should be 2:", allNames)
		return 0, ""
	}

	convertFunc := allNames[0]
	convertUnit := allNames[1]

	output := unitsFunctionsOutput(convertFunc, substance, unitInputs...)

	return output, convertUnit
}
