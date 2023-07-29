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
const altNamesSubsTableName string = "substanceNames"
const altNamesRouteTableName string = "routeNames"
const altNamesUnitsTableName string = "unitsNames"
const altNamesConvUnitsTableName string = "convUnitsNames"

// Read the config file for matching names and return the proper struct.
//
// nameType - choose between getting alt names for: substance, route, units or
// convUnits (conversion units)
//
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
		return errors.New(sprintName(printN, err)), nil
	}

	err = toml.Unmarshal(file, &subName)
	if err != nil {
		return errors.New(sprintName(printN, "Unmarshal error:", err)), nil
	}

	return nil, &subName
}

func namesTables(nameType string) (error, string) {
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
		return errors.New(sprintName(printN, "No nameType:", nameType)), ""
	}

	return nil, table
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
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// nameType - which table type to create: substance, route, units or
// convUnits (conversion units)
//
// sourceNames - if true, will add data to the source specific config tables
//
// overwrite - force overwrite of directory and tables
func (cfg Config) AddToNamesTable(db *sql.DB, ctx context.Context,
	nameType string, sourceNames bool, overwrite bool) error {
	const printN string = "AddToNamesTable()"

	err, setdir := InitSettingsDir()
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	var CopyToPath string = setdir + "/" + allNamesConfigsDir

	if overwrite == true {
		err := cfg.CleanNamesTables(db, ctx, false)
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

	err, table := namesTables(nameType)
	if err != nil {
		return errors.New(sprintName(printN, err))
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

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New(sprintName(printN, err))
	}

	subsStmt, err := tx.Prepare("insert into " + table +
		" (localName, alternativeName) " +
		"values(?, ?)")
	err = handleErrRollbackSeq(err, tx, printN, "tx.Prepare(): ")
	if err != nil {
		return err
	}
	defer subsStmt.Close()

	_, err = tx.Stmt(subsStmt).Exec(namesMagicWord, namesMagicWord)
	err = handleErrRollbackSeq(err, tx, printN, "tx.Stmt.Exec(): ")
	if err != nil {
		return err
	}

	for locName, altNames := range namesCfg.LocalName {
		locName = strings.ReplaceAll(locName, "_", " ")
		altName := altNames.AltNames
		for i := 0; i < len(altName); i++ {
			_, err = tx.Stmt(subsStmt).Exec(locName, altName[i])
			err = handleErrRollbackSeq(err, tx, printN, "tx.Stmt.Exec(): ")
			if err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	err = handleErrRollbackSeq(err, tx, printN, "tx.Commit(): ")
	if err != nil {
		return err
	}

	printName(printN, nameType, "names initialized successfully! sourceNames:", sourceNames)

	return nil
}

// MatchName replaces an input name with a configured output name present in
// the database. For example if there's a need to translate
// "weed" to "cannabis". Checkout AddToSubstanceNamesTable() for information
// on how the configuration is done.
//
// db - open database connection
//
// ctx - context to passed to sql query function
//
// inputName - the alternative name
//
// nameType - checkout namesTables()
//
// sourceNames - if true, it will use the config for the source,
// meaning the names specific for the source
//
// overwrite - if true will overwrite the names config directory
// and tables with the currently present ones,
// checkout AddToSubstanceNamesTable() for more info
//
// Returns the local name for a given alternative name.
func (cfg Config) MatchName(db *sql.DB, ctx context.Context, inputName string,
	nameType string, sourceNames bool, overwrite bool) string {
	const printN string = "MatchName()"

	_, table := namesTables(nameType)
	if table == "" {
		return inputName
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	err := cfg.AddToNamesTable(db, ctx, nameType, sourceNames, overwrite)
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
// Checkout MatchName()
//
// db - open database connection
//
// ctx - context to passed to sql query function
//
// inputName - the alternative name
//
// nameType - checkout namesTables()
func (cfg Config) MatchAndReplace(db *sql.DB, ctx context.Context,
	inputName string, nameType string) string {
	ret := cfg.MatchName(db, ctx, inputName, nameType, false, false)
	ret = cfg.MatchName(db, ctx, ret, nameType, true, false)
	return ret
}

// Tries matching a single string to all alternative names tables.
// If it finds a match it will return the alt name for that single string.
// It matches all alt drugs, route and units, so a single input can be checked
// for all of them. Checkout MatchAndReplace()
//
// db - open database connection
//
// ctx - context to passed to sql query function
//
// inputName - single string to match all alt names for
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

// Returns all alternative names for a given local name. For example if
// the input is "cannabis" it should return something like "weed", "marijuana",
// etc. The alt names themselves can't be used to find the other alt names,
// it requires the "main" name in the local info table.
//
// This function is meant to be run concurrently.
//
// db - open database connection
//
// ctx - context to passed to sql query function
//
// namesErrChan - the goroutine channel used to return the alternative names for
// a given "global" name and the error
//
// inputName - local name to get alt names for
//
// nameType - choose between getting alt names for: substance, route, units or
// convUnits (conversion units)
//
// sourceNames - use source specific names instead of global ones
func (cfg Config) GetAllNames(db *sql.DB, ctx context.Context,
	namesErrChan chan<- DrugNamesError, inputName string,
	nameType string, sourceNames bool) {
	const printN string = "GetAllNames()"

	tempDrugNamesErr := DrugNamesError{
		DrugNames: nil,
		Err:       nil,
	}

	err, table := namesTables(nameType)
	if err != nil {
		tempDrugNamesErr.Err = errors.New(sprintName(printN, err))
		namesErrChan <- tempDrugNamesErr
		return
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	cfg.AddToNamesTable(db, ctx, nameType, sourceNames, false)

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
		err = errors.New(sprintName(printN, "Error: ", err))
		tempDrugNamesErr.Err = err
		namesErrChan <- tempDrugNamesErr
		return
	}

	for rows.Next() {
		err = rows.Scan(&tempName)
		if err != nil {
			err = errors.New(sprintName(printN, "Scan: Error:", err))
			tempDrugNamesErr.Err = err
			namesErrChan <- tempDrugNamesErr
			return
		}
		allNames = append(allNames, tempName)
	}

	tempDrugNamesErr.DrugNames = allNames
	namesErrChan <- tempDrugNamesErr
	return
}

// Converts percentage to pure substance.
// input 0 - the total dose
// input 1 - the percentage
// output - pure substance calculated using the percentage
func convPerc2Pure(substance string, unitInputs ...float32) (error, float32) {
	av := alconvert.NewAV()
	av.UserSet.Milliliters = unitInputs[0]
	av.UserSet.Percent = unitInputs[1]
	av.CalcPureAmount()
	return nil, av.GotPure()
}

// Converts pure amount to grams.
// input 0 - the total dose
// input 1 - the percentage
// output - pure substance ml converted to grams using a constant density
func convMl2Grams(substance string, unitInputs ...float32) (error, float32) {
	const printN string = "convMl2Grams()"

	var multiplier float32 = 0

	// g/sm3
	substancesDensities := map[string]float32{
		"Alcohol": 0.79283, // At 16 C temperature
	}

	multiplier = substancesDensities[substance]

	if multiplier == 0 {
		err := errors.New(sprintName(printN, "No density for substance:", substance))
		return err, 0
	}
	_, finalRes := convPerc2Pure(substance, unitInputs...)
	finalRes = finalRes * multiplier
	return nil, finalRes
}

type convF func(string, ...float32) (error, float32)

func addConversion(cF convF, output float32, name string, inputName string, inputsAmount int, substance string, unitInputs ...float32) (error, float32) {
	if output != 0 {
		return nil, output
	}

	const printN string = "addConversion()"

	var err error = nil

	if inputName == name {
		gotLenOfUnitInputs := len(unitInputs)
		if gotLenOfUnitInputs == inputsAmount {
			err, output = cF(substance, unitInputs...)
		} else {
			err = errors.New(sprintName(printN, "Wrong amount of unitInputs:",
				gotLenOfUnitInputs, "; needed", inputsAmount))
		}
	}

	return err, output
}

func unitsFunctionsOutput(inputName string, substance string, unitInputs ...float32) (error, float32) {
	err, output := addConversion(convPerc2Pure, 0, "Convert-Percent-To-Pure", inputName, 2, substance, unitInputs...)
	if err != nil {
		return err, output
	}

	err, output = addConversion(convMl2Grams, output, "Convert-Milliliters-To-Grams", inputName, 2, substance, unitInputs...)
	if err != nil {
		return err, output
	}

	return err, output
}

// ConvertUnits converts the given inputs for a given substance according to a
// predefined configuration in the database. Checkout
// AddToSubstanceNamesTable() for more info on how the configuration is done.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// substance - the drug for which to convert units via the config
//
// unitInputs - the inputs to use for the conversions, for example
// milliliters and percentage
func (cfg Config) ConvertUnits(db *sql.DB, ctx context.Context, substance string, unitInputs ...float32) (error, float32, string) {
	const printN string = "ConvertUnits()"

	drugNamesErrChan := make(chan DrugNamesError)
	go cfg.GetAllNames(db, ctx, drugNamesErrChan, substance, "convUnits", true)
	gotDrugNamesErr := <-drugNamesErrChan
	allNames := gotDrugNamesErr.DrugNames
	err := gotDrugNamesErr.Err
	if err != nil {
		err = errors.New(sprintName(printN, err))
		return err, 0, ""
	}
	gotAllNamesLen := len(allNames)
	if gotAllNamesLen != 2 {
		err := errors.New(sprintName(printN, "Wrong amount of names:", gotAllNamesLen,
			"; should be 2:", allNames))
		return err, 0, ""
	}

	convertFunc := allNames[0]
	convertUnit := allNames[1]

	err, output := unitsFunctionsOutput(convertFunc, substance, unitInputs...)

	return err, output, convertUnit
}
