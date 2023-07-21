package drugdose

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/powerjungle/goalconvert/alconvert"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "github.com/glebarez/go-sqlite"

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

// Constants used for matching names
const nameTypeSubstance = "substance"
const nameTypeRoute = "route"
const nameTypeUnits = "units"
const nameTypeConvertUnits = "convUnits"

// Read the config file for matching names and return the proper struct.
// nameType - checkout namesFiles()
// source - if not empty, will read the source specific config
func GetNamesConfig(nameType string, source string) (error, *SubstanceName) {
	const printN string = "GetNamesConfig()"

	err, setdir := InitSettingsDir()
	if err != nil {
		return errors.New(sprintName(printN, err)), nil
	}

	err, gotFile := namesFiles(nameType)
	if err != nil {
		return errors.New(sprintName(printN, err)), nil
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
			return errors.New(sprintName(printN, err)), nil
		}
		return nil, nil
	}

	err = toml.Unmarshal(file, &subName)
	if err != nil {
		return errors.New(sprintName(printN, "Unmarshal error:", err)), nil
	}

	return nil, &subName
}

func namesTables(nameType string) string {
	const printN string = "namesTables()"

	table := ""
	if nameType == nameTypeSubstance {
		table = altNamesSubsTableName
	} else if nameType == nameTypeRoute {
		table = altNamesRouteTableName
	} else if nameType == nameTypeUnits {
		table = altNamesUnitsTableName
	} else if nameType == nameTypeConvertUnits {
		table = altNamesConvUnitsTableName
	} else {
		printName(printN, "No nameType:", nameType)
	}

	return table
}

func namesFiles(nameType string) (error, string) {
	const printN string = "namesFiles()"

	file := ""
	if nameType == nameTypeSubstance {
		file = namesSubstanceFilename
	} else if nameType == nameTypeRoute {
		file = namesRouteFilename
	} else if nameType == nameTypeUnits {
		file = namesUnitsFilename
	} else if nameType == nameTypeConvertUnits {
		file = namesConvUnitsFilename
	} else {
		return errors.New(sprintName(printN, "No nameType:", nameType)), ""
	}

	return nil, file
}

// Copy the config files to the proper directory and read them to
// create the proper tables in the database, which will later be used
// to match alternative names to local names.
//
// nameType - checkout namesTables() and namesFiles()
//
// sourceNames - if true, will add data to the source specific config tables
//
// overwrite - force overwrite of directory and tables
func (cfg Config) AddToSubstanceNamesTable(db *sql.DB, ctx context.Context,
	nameType string, sourceNames bool, overwrite bool) error {
	const printN string = "AddToSubstanceNamesTable()"

	err, setdir := InitSettingsDir()
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	var CopyToPath string = setdir + "/" + allNamesConfigsDir

	if overwrite == true {
		err := cfg.CleanNames(db, ctx, false)
		if err != nil {
			return errors.New(sprintName(printN,
				"Couldn't clean names from database for overwrite: ", err))
		}

		err = cfg.InitAllDBTables(db, ctx)
		if err != nil {
			return errors.New(sprintName(printN,
				"Database didn't get initialised, because of an error: ", err))
		}

		err = os.RemoveAll(CopyToPath)
		if err != nil {
			return errors.New(sprintName(printN, err))
		} else {
			printName(printN, "Deleted directory:", CopyToPath)
		}
	}

	table := namesTables(nameType)
	if table == "" {
		return errors.New(sprintName(printN, "No tables returned."))
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	ret := checkIfExistsDB(db, ctx,
		"localName",
		table,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		namesMagicWord)
	if ret {
		return nil
	}

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
					return errors.New(sprintName(printN, "Couldn't move file:", err))
				} else if err == nil {
					printName(printN, "Done copying to:", CopyToPath)
				}
			} else {
				return errors.New(sprintName(printN, err))
			}
		} else {
			return errors.New(sprintName(printN, err))
		}
	} else if err == nil {
		printNameVerbose(cfg.VerbosePrinting, printN, "Name config already exists:", CopyToPath,
			"; will not copy the config directory from the working directory:", allNamesConfigsDir)
	}

	getCfgSrc := ""
	if sourceNames {
		getCfgSrc = cfg.UseSource
	}

	err, namesCfg := GetNamesConfig(nameType, getCfgSrc)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	subsStmt, err := db.PrepareContext(ctx, "insert into "+table+
		" (localName, alternativeName) "+
		"values(?, ?)")
	if err != nil {
		return errors.New(sprintName(printN, err))
	}
	defer subsStmt.Close()

	tx, err := db.Begin()
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	_, err = tx.Stmt(subsStmt).ExecContext(ctx, namesMagicWord, namesMagicWord)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	for locName, altNames := range namesCfg.LocalName {
		locName = strings.ReplaceAll(locName, "_", " ")
		altName := altNames.AltNames
		for i := 0; i < len(altName); i++ {
			_, err = tx.Stmt(subsStmt).ExecContext(ctx, locName, altName[i])
			if err != nil {
				return errors.New(sprintName(printN, err))
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	printName(printN, nameType, "names initialized successfully! sourceNames:", sourceNames)

	return nil
}

// db - open database connection
//
// ctx - context to passed to sql query function
//
// inputName - the alternative name
//
// nameType - checkout namesTables()
//
// sourceNames - if true, it will use the config for the source,
//
// meaning the names specific for the source
//
// overwrite - if true will overwrite the names config directory and tables with the currently present ones
//
// Returns the local name for a given alternative name.
func (cfg Config) MatchName(db *sql.DB, ctx context.Context, inputName string,
	nameType string, sourceNames bool, overwrite bool) string {
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

	err := cfg.AddToSubstanceNamesTable(db, ctx, nameType, sourceNames, overwrite)
	if err != nil {
		return inputName
	}

	checkCol := []string{"localName", "alternativeName"}
	var gotDBName string
	for i := 0; i < len(checkCol); i++ {
		gotDBName = ""
		err := db.QueryRowContext(ctx, "select localName from "+table+
			" where "+checkCol[i]+" = ?", inputName).Scan(&gotDBName)
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
func (cfg Config) MatchAndReplace(db *sql.DB, ctx context.Context,
	inputName string, nameType string) string {
	ret := cfg.MatchName(db, ctx, inputName, nameType, false, false)
	ret = cfg.MatchName(db, ctx, ret, nameType, true, false)
	return ret
}

// Tries matching a single string to all alternative names.
func (cfg Config) MatchAndReplaceAll(db *sql.DB, ctx context.Context, inputName string) string {
	allNameTypes := []string{nameTypeSubstance, nameTypeRoute, nameTypeUnits, nameTypeConvertUnits}
	for _, elem := range allNameTypes {
		retName := cfg.MatchAndReplace(db, ctx, inputName, elem)
		if retName != inputName {
			return retName
		}
	}
	return inputName
}

// Returns all alternative names for a given local name.
func (cfg Config) GetAllNames(db *sql.DB, ctx context.Context, inputName string, nameType string, sourceNames bool) []string {
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

	cfg.AddToSubstanceNamesTable(db, ctx, nameType, sourceNames, false)

	repName := cfg.MatchName(db, ctx, inputName, nameType, true, false)
	if repName != inputName {
		printNameVerbose(cfg.VerbosePrinting, printN, "For source:", cfg.UseSource,
			"; Local name:", inputName, "; Is sourceNamesd with:", repName)
	}

	var allNames []string
	var tempName string
	rows, err := db.QueryContext(ctx, "select alternativeName from "+table+
		" where localName = ?", repName)
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

func (cfg Config) ConvertUnits(db *sql.DB, ctx context.Context, substance string, unitInputs ...float32) (float32, string) {
	const printN string = "ConvertUnits()"

	allNames := cfg.GetAllNames(db, ctx, substance, "convUnits", true)
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
