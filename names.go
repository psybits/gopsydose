package drugdose

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/powerjungle/goalconvert/alconvert"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"
	// SQLite driver needed for sql module
	_ "modernc.org/sqlite"
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
const NameTypeSubstance = "substance"
const NameTypeRoute = "route"
const NameTypeUnits = "units"
const NameTypeConvertUnits = "convUnits"
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
		return fmt.Errorf("%s%w", sprintName(printN), err), nil
	}

	err, gotFile := namesFiles(nameType)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err), nil
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
		return fmt.Errorf("%s%w", sprintName(printN), err), nil
	}

	err = toml.Unmarshal(file, &subName)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN, "toml.Unmarshal(): "), err), nil
	}

	return nil, &subName
}

func namesTables(nameType string) (error, string) {
	const printN string = "namesTables()"

	table := ""
	if nameType == NameTypeSubstance {
		table = altNamesSubsTableName
	} else if nameType == NameTypeRoute {
		table = altNamesRouteTableName
	} else if nameType == NameTypeUnits {
		table = altNamesUnitsTableName
	} else if nameType == NameTypeConvertUnits {
		table = altNamesConvUnitsTableName
	} else {
		return fmt.Errorf("%s%w: %s", sprintName(printN), NoNametypeError, nameType), ""
	}

	return nil, table
}

func namesFiles(nameType string) (error, string) {
	const printN string = "namesFiles()"

	file := ""
	if nameType == NameTypeSubstance {
		file = namesSubstanceFilename
	} else if nameType == NameTypeRoute {
		file = namesRouteFilename
	} else if nameType == NameTypeUnits {
		file = namesUnitsFilename
	} else if nameType == NameTypeConvertUnits {
		file = namesConvUnitsFilename
	} else {
		return fmt.Errorf("%s%w: %s", sprintName(printN), NoNametypeError, nameType), ""
	}

	return nil, file
}

// Create the proper tables in the database, which will later be used
// to match alternative names to local names.
//
// db - open database connection
//
// ctx - context to be passed to sql queries
//
// nameType - choose type for table to create, between exported constants:
// NameTypeSubstance, NameTypeRoute, NameTypeUnits or NameTypeConvertUnits
//
// sourceNames - if true, will add data to the source specific config tables
func (cfg Config) AddToNamesTable(db *sql.DB, ctx context.Context,
	nameType string, sourceNames bool) error {
	const printN string = "AddToNamesTable()"

	err, table := namesTables(nameType)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
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

	getCfgSrc := ""
	if sourceNames {
		getCfgSrc = cfg.UseSource
	}

	err, namesCfg := GetNamesConfig(nameType, getCfgSrc)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s%w", sprintName(printN), err)
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

// Calls AddToNamesTable() for all nameType.
//
// overwrite - force overwrite of tables, it will not remove
// the old config files, that must be done manually, if they're not removed
// it will use their data for the database
func (cfg Config) AddToAllNamesTables(db *sql.DB, ctx context.Context,
	overwrite bool) error {

	const printN string = "AddToAllNamesTables()"

	if overwrite == true {
		err := cfg.CleanNamesTables(db, ctx, false)
		if err != nil {
			return fmt.Errorf("%s%w", sprintName(printN), err)
		}

		err = cfg.InitAllDBTables(db, ctx)
		if err != nil {
			return fmt.Errorf("%s%w", sprintName(printN), err)
		}
	}

	nameTypes := [4]string{NameTypeSubstance, NameTypeRoute,
		NameTypeUnits, NameTypeConvertUnits}

	// Add to global names tables
	for i := 0; i < 4; i++ {
		err := cfg.AddToNamesTable(db, ctx, nameTypes[i], false)
		if err != nil && errors.Is(err, fs.ErrNotExist) == false {
			return fmt.Errorf("%s%w", sprintName(printN), err)
		}
	}

	// Add to source names tables
	for i := 0; i < 4; i++ {
		err := cfg.AddToNamesTable(db, ctx, nameTypes[i], true)
		if err != nil && errors.Is(err, fs.ErrNotExist) == false {
			return fmt.Errorf("%s%w", sprintName(printN), err)
		}
	}

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
// nameType - choose type to replace, between exported constants: NameTypeSubstance,
// NameTypeRoute, NameTypeUnits or NameTypeConvertUnits
//
// sourceNames - if true, it will use the config for the source,
// meaning the names specific for the source
//
// Returns the local name for a given alternative name.
func (cfg Config) MatchName(db *sql.DB, ctx context.Context,
	inputName string, nameType string, sourceNames bool) string {
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

	// Check localName first, in case the inputName matches it, to avoid
	// unnecessary work looking for it at the alternativeName column.
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

// Returns the local name, using both the global config and the
// source specific config. Checkout MatchName()
//
// db - open database connection
//
// ctx - context to passed to sql query function
//
// inputName - the alternative name
//
// nameType - choose type to replace, between exported constants: NameTypeSubstance,
// NameTypeRoute, NameTypeUnits or NameTypeConvertUnits
func (cfg Config) MatchAndReplace(db *sql.DB, ctx context.Context,
	inputName string, nameType string) string {
	ret := cfg.MatchName(db, ctx, inputName, nameType, false)
	ret = cfg.MatchName(db, ctx, ret, nameType, true)
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
func (cfg Config) MatchAndReplaceAll(db *sql.DB, ctx context.Context,
	inputName string) string {

	allNameTypes := []string{NameTypeSubstance, NameTypeRoute,
		NameTypeUnits, NameTypeConvertUnits}
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
// nameType - choose type to get alt names for, between exported constants:
// NameTypeSubstance, NameTypeRoute, NameTypeUnits or NameTypeConvertUnits
//
// sourceNames - use source specific names instead of global ones
//
// username - the user requesting the alt names
func (cfg Config) GetAllAltNames(db *sql.DB, ctx context.Context,
	namesErrChan chan<- DrugNamesError, inputName string,
	nameType string, sourceNames bool, username string) {
	const printN string = "GetAllAltNames()"

	tempDrugNamesErr := DrugNamesError{
		DrugNames: nil,
		Username:  "",
		Err:       nil,
	}

	err, table := namesTables(nameType)
	if err != nil {
		tempDrugNamesErr.Err = fmt.Errorf("%s%w", sprintName(printN), err)
		namesErrChan <- tempDrugNamesErr
		return
	}

	tableSuffix := ""
	if sourceNames {
		tableSuffix = "_" + cfg.UseSource
	}

	table = table + tableSuffix

	repName := cfg.MatchName(db, ctx, inputName, nameType, true)
	if repName != inputName {
		printNameVerbose(cfg.VerbosePrinting, printN, "For source:", cfg.UseSource,
			"; Local name:", inputName, "; Is sourceNamesd with:", repName)
	}

	var allNames []string
	var tempName string
	rows, err := db.QueryContext(ctx, "select alternativeName from "+table+
		" where localName = ?", repName)
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
		tempDrugNamesErr.Err = err
		namesErrChan <- tempDrugNamesErr
		return
	}

	for rows.Next() {
		err = rows.Scan(&tempName)
		if err != nil {
			err = fmt.Errorf("%s%w", sprintName(printN), err)
			tempDrugNamesErr.Err = err
			namesErrChan <- tempDrugNamesErr
			return
		}
		allNames = append(allNames, tempName)
	}

	if len(allNames) == 0 {
		tempDrugNamesErr.Err = fmt.Errorf("%s%w: %s", sprintName(printN),
			NoNamesReturnedError, " for "+nameType+": "+inputName)
	}

	tempDrugNamesErr.DrugNames = allNames
	tempDrugNamesErr.Username = username
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
		err := fmt.Errorf("%s%w: %s", sprintName(printN), NoDensitySubstanceError, substance)
		return err, 0
	}
	_, finalRes := convPerc2Pure(substance, unitInputs...)
	finalRes = finalRes * multiplier
	return nil, finalRes
}

type convF func(string, ...float32) (error, float32)

func addConversion(cF convF, output float32, name string,
	inputName string, inputsAmount int, substance string,
	unitInputs ...float32) (error, float32) {

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
			err = fmt.Errorf("%s%w: %q ; needed: %q", sprintName(printN), WrongAmountUnitInputsError,
				gotLenOfUnitInputs, inputsAmount)
		}
	}

	return err, output
}

func unitsFunctionsOutput(inputName string, substance string, unitInputs ...float32) (error, float32) {
	err, output := addConversion(convPerc2Pure, 0,
		"Convert-Percent-To-Pure", inputName, 2, substance, unitInputs...)
	if err != nil {
		return err, output
	}

	err, output = addConversion(convMl2Grams, output,
		"Convert-Milliliters-To-Grams", inputName, 2, substance, unitInputs...)
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
func (cfg Config) ConvertUnits(db *sql.DB, ctx context.Context,
	substance string, unitInputs ...float32) (error, float32, string) {

	const printN string = "ConvertUnits()"

	substance = cfg.MatchAndReplace(db, ctx, substance, NameTypeSubstance)

	drugNamesErrChan := make(chan DrugNamesError)
	go cfg.GetAllAltNames(db, ctx, drugNamesErrChan, substance, "convUnits", true, "")
	gotDrugNamesErr := <-drugNamesErrChan
	allNames := gotDrugNamesErr.DrugNames
	err := gotDrugNamesErr.Err
	if err != nil {
		err = fmt.Errorf("%s%w", sprintName(printN), err)
		return err, 0, ""
	}
	gotAllNamesLen := len(allNames)
	if gotAllNamesLen != 2 {
		err := fmt.Errorf("%s%w: %q ; needed: %q", sprintName(printN), WrongAmountNamesError,
			gotAllNamesLen, allNames)
		return err, 0, ""
	}

	convertFunc := allNames[0]
	convertUnit := allNames[1]

	err, output := unitsFunctionsOutput(convertFunc, substance, unitInputs...)

	if output == 0 || convertUnit == "" || err != nil {
		if err == nil && output == 0 {
			err = ConvResultIsZeroError
		} else if err == nil && convertFunc == "" {
			err = RetConvertUnitEmptyError
		}
		err = fmt.Errorf("%sError converting units for drug: %q"+
			" ; dose: %g ; units: %q ; error: %w",
			sprintName(printN), substance, output, convertUnit, err)
	}

	return err, output, convertUnit
}

var NoNametypeError error = errors.New("no nameType")
var NoNamesReturnedError error = errors.New("no names returned")
var NoDensitySubstanceError error = errors.New("got no density for substance")
var WrongAmountUnitInputsError error = errors.New("wrong amount of unitInputs")
var WrongAmountNamesError error = errors.New("wrong amount of names")
var ConvResultIsZeroError error = errors.New("conversion result is zero")
var RetConvertUnitEmptyError error = errors.New("returned convertUnit is empty")
