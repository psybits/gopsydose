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
)

type AlternativeNames struct {
	AltNames []string
}

type SubstanceName struct {
	LocalName map[string]AlternativeNames
}

const namesSubstanceFilename = "gpd-substance-names.toml"
const namesRouteFilename = "gpd-route-names.toml"
const namesUnitsFilename = "gpd-units-names.toml"
const namesConvUnitsFilename = "gpd-units-conversions.toml"

const replaceDir = "replace-local-names-configs"

const namesMagicWord = "!TheTableIsNotEmpty!"

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
		gotFile = replaceDir + "/" + source + "/" + gotFile
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

func (cfg Config) AddToSubstanceNamesTable(nameType string, replace bool) bool {
	const printN string = "AddToSubstanceNamesTable()"

	table := namesTables(nameType)
	if table == "" {
		return false
	}

	tableSuffix := ""
	if replace {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	file := namesFiles(nameType)
	if file == "" {
		return false
	}

	ret := checkIfExistsDB("localName",
		table,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		namesMagicWord)
	if ret {
		return true
	}

	setdir := InitSettingsDir()
	if setdir == "" {
		printName(printN, "No settings directory found!")
		return false
	}
	paths := [2]string{file, replaceDir}
	for i := 0; i < len(paths); i++ {
		// Check if files exist in current working directory.
		// If they do, try to move them to the config directory.
		_, err := os.Stat(paths[i])
		if err == nil {
			moveToPath := setdir + "/" + paths[i]

			printName(printN, "Found config in working directory:", paths[i],
				"; attempt moving to:", moveToPath)

			// Check if files exist in config directory.
			// If they don't, move them to config directory.
			// If they do, don't move them, because you will overwrite the old files.
			_, err := os.Stat(moveToPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					err = os.Rename(paths[i], moveToPath)
					if err != nil {
						printName(printN, "Couldn't move file:", err)
						return false
					}
				} else {
					printName(printN, err)
					return false
				}
			} else if err == nil {
				printName(printN, "Name config already exists:", moveToPath,
					"; will not move the file from the working directory:", paths[i])
			}
		} else if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				printName(printN, err)
				return false
			}
		}
	}

	getCfgSrc := ""
	if replace {
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

	printNameVerbose(cfg.VerbosePrinting, printN, nameType, "names initialized successfully! replace:", replace)

	return true
}

func (cfg Config) MatchName(inputName string, nameType string, replace bool) string {
	const printN string = "MatchName()"

	table := namesTables(nameType)
	if table == "" {
		return inputName
	}

	tableSuffix := ""
	if replace {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	ret := cfg.AddToSubstanceNamesTable(nameType, replace)
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

func (cfg Config) MatchAndReplace(inputName string, nameType string) string {
	ret := cfg.MatchName(inputName, nameType, false)
	ret = cfg.MatchName(ret, nameType, true)
	return ret
}

func (cfg Config) GetAllNames(inputName string, nameType string, replace bool) []string {
	const printN string = "GetAllNames()"

	table := namesTables(nameType)
	if table == "" {
		return nil
	}

	tableSuffix := ""
	if replace {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	cfg.AddToSubstanceNamesTable(nameType, replace)

	repName := cfg.MatchName(inputName, nameType, true)
	if repName != inputName {
		printNameVerbose(cfg.VerbosePrinting, printN, "For source:", cfg.UseSource,
			"; Local name:", inputName, "; Is replaced with:", repName)
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
	if replace {
		addToErrMsg = "; for source: " + cfg.UseSource
	}

	if len(allNames) == 0 {
		printName(printN, "No names found for:", repName, addToErrMsg)
	}

	return allNames
}

func convPerc2Units(amount float32, perc float32) float32 {
	av := alconvert.NewAV()
	av.UserSet.Milliliters = amount
	av.UserSet.Percent = perc
	av.CalcGotUnits()
	return av.GotUnits()
}

func unitsFunctionsOutput(convertFunc string, unitInputs ...float32) float32 {
	const printN string = "unitsFunctionsOutput()"

	var output float32 = 0
	if convertFunc == "ConvPerc2Units" || convertFunc == "ConvPerc2Units*10" {
		gotLenOfUnitInputs := len(unitInputs)
		if gotLenOfUnitInputs == 2 {
			output = convPerc2Units(unitInputs[0], unitInputs[1])
			if convertFunc == "ConvPerc2Units*10" {
				output *= 10
			}
		} else {
			printName(printN, "ConvPerc2Units: Wrong amount of unitInputs:",
				gotLenOfUnitInputs, "; needed 2")
		}
	} else {
		printName(printN, "No convertFunc:", convertFunc)
	}

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

	output := unitsFunctionsOutput(convertFunc, unitInputs...)

	return output, convertUnit
}
