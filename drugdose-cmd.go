package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/psybits/gopsydose/drugdose"
)

var (
	drugname = flag.String(
		"drug",
		"none",
		"The name of the drug.\nTry to be as accurate as possible!")

	drugroute = flag.String(
		"route",
		"none",
		"oral, smoked, sublingual, insufflation, inhalation,\n"+
			"intravenous, etc.")

	drugargdose = flag.Float64(
		"dose",
		0,
		"just a number, without any units such as ml around it")

	drugunits = flag.String(
		"units",
		"none",
		"the units themselves: ml, L, mg etc.")

	drugperc = flag.Float64(
		"perc",
		0,
		"this is only used for alcohol currently,\n"+
			"again just a number, no % around it")

	drugcost = flag.Float64(
		"cost",
		0,
		"the cost in money for the logged dose,\n"+
			"you have to calculate using the total you paid")

	costCur = flag.String(
		"cost-cur",
		"",
		"the currency to be used for the cost")

	changeLog = flag.Bool(
		"change-log",
		false,
		"make changes to an entry/log/dosage,\n"+
			"if -for-id is not specified, it's to the last entry\n"+
			"must be used in combination with:\n"+
			"-end-time ; -start-time ; -drug\n"+
			"in order to clarify what you want to change in the entry")

	endTime = flag.String(
		"end-time",
		"none",
		"change the end time of the last log\n"+
			"it accepts unix timestamps as input\n"+
			"if input is the string \"now\"\n"+
			"it will use the current time\n\n"+
			"must be used in combination with -change-log\n"+
			"if combined with -for-id as well\n"+
			"it will change for a specific ID")

	startTime = flag.String(
		"start-time",
		"none",
		"change the start time of the last log\n"+
			"it accepts unix timestamps as input\n"+
			"if input is the string \"now\"\n"+
			"it will use the current time\n\n"+
			"must be used in combination with -change-log\n"+
			"if combined with -for-id as well\n"+
			"it will change for a specific ID")

	forUser = flag.String(
		"user",
		drugdose.DefaultUsername,
		"log for a specific user, for example if you're looking\n"+
			"after a friend")

	getNewLogs = flag.Int(
		"get-new-logs",
		0,
		"print the N number of the newest logs for the current user")

	getOldLogs = flag.Int(
		"get-old-logs",
		0,
		"print the N number of the oldest logs for the current user")

	getLogs = flag.Bool(
		"get-logs",
		false,
		"print all logs for the current user")

	getLogsCount = flag.Bool(
		"get-logs-count",
		false,
		"print the total number of logs for the current user")

	removeNew = flag.Int(
		"clean-new-logs",
		0,
		"cleans the N number of newest logs")

	removeOld = flag.Int(
		"clean-old-logs",
		0,
		"cleans the N number of oldest logs")

	cleanDB = flag.Bool(
		"clean-db",
		false,
		"remove all tables from the DB")

	cleanLogs = flag.Bool(
		"clean-logs",
		false,
		"cleans the logs\noptionally using the -user option for\n"+
			"clearing logs for a specific user")

	cleanNames = flag.Bool(
		"clean-names",
		false,
		"cleans the alternative names from the DB\n"+
			"this includes the replace names for the\n"+
			"currently configured source\n"+
			"when you reuse the name matching, it will\n"+
			"recreate the tables with the present config files")

	overwriteNames = flag.Bool(
		"overwrite-names",
		false,
		"overwrite the alternative names in the DB,\n"+
			"it will delete the old directory and tables\n"+
			"and replace them with the currently present ones")

	cleanInfo = flag.Bool(
		"clean-info",
		false,
		"cleans the currently configured info table,\n"+
			"meaning all remotely fetched dosage ranges and routes\n"+
			"for all drugs, keep in mind that if you have configured a\n"+
			"different source earlier, it will not be cleaned, unless\n"+
			"you change the configuration back and use this flag again")

	forID = flag.Int64(
		"for-id",
		0,
		"perform and action for a particular id")

	sourcecfg = flag.String(
		"sourcecfg",
		drugdose.DefaultSource,
		"the name of the API that you want to initialise for\n"+
			"settings and sources config files")

	apiAddress = flag.String(
		"api-address",
		drugdose.DefaultAPI,
		"the address of the API that you want to initialise for\n"+
			"sources config file combined with -source")

	recreateSettings = flag.Bool(
		"recreate-settings",
		false,
		"recreate the settings.toml file")

	recreateSources = flag.Bool(
		"recreate-sources",
		false,
		"recreate the sources.toml file")

	dbDir = flag.String(
		"db-dir",
		drugdose.DefaultDBDir,
		"full path of the DB directory, this will work only on\n"+
			"the initial run, you can change it later in the\n"+
			"settings config file, don't forget to delete\n"+
			"the old DB directory")

	getDirs = flag.Bool(
		"get-dirs",
		false,
		"prints the settings and DB directories path")

	getLocalInfoDrug = flag.String(
		"get-local-info-drug",
		"none",
		"print info about drug from local DB,\n"+
			"for example if you've forgotten routes and units")

	getLocalInfoDrugs = flag.Bool(
		"get-local-info-drugs",
		false,
		"get all unique drug names (from info tables, not logs)\n"+
			"according to set source")

	getLoggedDrugs = flag.Bool(
		"get-logged-drugs",
		false,
		"get all unique drug names (from log tables, not info)\n"+
			"using the provided username")

	getTotalCosts = flag.Bool(
		"get-total-costs",
		false,
		"print all costs for all drugs in all currencies")

	getSubNames = flag.String(
		"get-subst-alt-names",
		"",
		"get all alternative names for a substance")

	getRouteNames = flag.String(
		"get-route-alt-names",
		"",
		"get all alternative names for a route")

	getUnitsNames = flag.String(
		"get-units-alt-names",
		"",
		"get all alternative names for a unit")

	dontLog = flag.Bool(
		"dont-log",
		false,
		"only fetch info about drug to local DB,\n"+
			"but don't log anything")

	removeInfoDrug = flag.String(
		"remove-info-drug",
		"none",
		"remove all entries of a single drug from the info DB")

	getTimes = flag.Bool(
		"get-times",
		false,
		"get the times till onset, comeup, etc.\n"+
			"according to the current time\n"+
			"can be combined with -for-id to\n"+
			"get times for a specific ID\n"+
			"use -get-logs, to see the IDs")

	getUsers = flag.Bool(
		"get-users",
		false,
		"get all usernames logged")

	getDBSize = flag.Bool(
		"get-db-size",
		false,
		"get total size in MiB and bytes for the\n"+
			"one database used for logging and drugs info")

	stopOnCfgInit = flag.Bool(
		"stop-on-config-init",
		false,
		"stops the program once the config\n"+
			"files have been initialised")

	stopOnDbInit = flag.Bool(
		"stop-on-db-init",
		false,
		"stops the program once the DB file has been created\n"+
			"and initialised, if it doesn't exists already")

	verbose = flag.Bool(
		"verbose",
		drugdose.DefaultVerbose,
		"print extra information")

	remember = flag.Bool(
		"remember",
		false,
		"remember last dose config")

	forget = flag.Bool(
		"forget",
		false,
		"forget the last set remember config")

	dbDriverInfo = flag.Bool(
		"DBDriverInfo",
		false,
		"show info about the current configured database driver")

	noGetLimit = flag.Bool(
		"no-get-limit",
		false,
		"there is a default limit of getting\n"+
			"a maximum of 100 entries, you can bypass it\n"+
			"using this option, this does not\n"+
			"affect logging new entries for that do -get-dirs\n"+
			"and checkout the settings file")

	searchStr = flag.String(
		"search",
		"none",
		"search all columns for specific string")

	searchExact = flag.Bool(
		"search-exact",
		false,
		"search for a specific column\n"+
			"the column you search for is dependent on\n"+
			"the -drug -route -units flags, you don't need\n"+
			"to use the -search flag,\n"+
			"compared to -search, this doesn't look if the\n"+
			"string is contained, but if it's exactly the same")
)

// Print strings properly formatted for the Command Line Interface (CLI) program.
// This is so that when using the CLI program, the user can better understand
// where a string is coming from.
// If you only need to add a newline, don't use this function!
func printCLI(str ...any) {
	fmt.Print("CLI: ")
	fmt.Println(str...)
}

// Same as printCLI(), but only for verbose output and is optional.
func printCLIVerbose(verbose bool, str ...any) {
	if verbose == true {
		printCLI(str...)
	}
}

func countExec(execCount *uint8) bool {
	if *execCount > 0 {
		*execCount -= 1
	}
	if *execCount <= 0 {
		return false
	}
	return true
}

// Count how many times a goroutine that uses the struct ErrorInfo has started
// and remove 1 from the total count every time data has been sent from the
// goroutine. When it reaches 0, stop the handler so that wg.Wait()
// at the end can stop. The reason for this counting is so that we know how
// many flags have been used and if we need to wait for them.
// It wasn't needed to use a handler for this program, but it's a good way
// of testing if it works properly and as a demo. Other programs don't have
// to count if they don't need to, every project has unique requirements and
// should take care of them in their own way.
func handleErrInfo(errInfo drugdose.ErrorInfo, a ...any) bool {
	if errInfo.Err != nil {
		printCLI(errInfo.Err)
	} else if errInfo.Err == nil && errInfo.Action != "" {
		printCLI(errInfo.Action)
	}
	return countExec(a[0].(*uint8))
}

func main() {
	// Initialisation /////////////////////////////////////////////////////
	flag.Parse()

	if flag.NFlag() == 0 {
		printCLI("Try adding -help with space next to the program name! You can read the README file as well.")
		os.Exit(1)
	}

	gotsetcfg := drugdose.InitAllSettings(*sourcecfg, *dbDir, drugdose.DefaultDBName,
		drugdose.DefaultMySQLAccess, *recreateSettings, *recreateSources,
		*verbose, *apiAddress)

	if *stopOnCfgInit {
		printCLI("Stopping after config file initialization.")
		os.Exit(0)
	}

	ctx, ctx_cancel, err := gotsetcfg.UseConfigTimeout()
	if err != nil {
		printCLI(err)
		os.Exit(1)
	}
	defer ctx_cancel()

	gotsetcfg.InitAllDB(ctx)

	if *stopOnDbInit {
		printCLI("Stopping after database initialization.")
		os.Exit(0)
	}

	db := gotsetcfg.OpenDBConnection(ctx)
	defer db.Close()

	var execCount uint8 = 1
	var wg sync.WaitGroup
	errInfoChanHandled := drugdose.AddChannelHandler(&wg, handleErrInfo, &execCount)
	///////////////////////////////////////////////////////////////////////

	setType := ""
	setValue := ""
	getExact := ""
	if *startTime != "none" {
		setType = "start-time"
		setValue = *startTime
	} else if *endTime != "none" {
		setType = "end-time"
		setValue = *endTime
	} else if *drugname != "none" {
		setType = drugdose.LogDrugNameCol
		setValue = *drugname
	} else if *drugargdose != 0 {
		setType = drugdose.LogDoseCol
		setValue = strconv.FormatFloat(*drugargdose, 'f', -1, 64)
	} else if *drugunits != "none" {
		setType = drugdose.LogDoseUnitsCol
		setValue = *drugunits
	} else if *drugroute != "none" {
		setType = drugdose.LogDrugRouteCol
		setValue = *drugroute
	} else if *drugcost != 0 {
		setType = drugdose.LogCostCol
		setValue = strconv.FormatFloat(*drugcost, 'f', -1, 64)
	} else if *costCur != "" {
		setType = drugdose.LogCostCurrencyCol
		setValue = *costCur
	}

	if *searchExact {
		getExact = setType
		*searchStr = setValue
	}

	if *getDirs {
		printCLI("DB Dir:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
		err, gotsetdir := drugdose.InitSettingsDir()
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
		printCLI("Settings Dir:", gotsetdir)
	}

	if *dbDriverInfo {
		printCLI("Using database driver:", gotsetcfg.DBDriver)
		printCLI("Database path:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
	}

	if *cleanDB {
		err := gotsetcfg.CleanDB(db, ctx)
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
	}

	if *cleanInfo {
		err := gotsetcfg.CleanInfoTable(db, ctx)
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
	}

	if *cleanNames {
		err := gotsetcfg.CleanNamesTables(db, ctx, false)
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
	}

	if *overwriteNames {
		gotsetcfg.MatchName(db, ctx, "asd", "substance", false, true)
	}

	if *forget {
		errInfoChan := make(chan drugdose.ErrorInfo)
		go gotsetcfg.ForgetDosing(db, ctx, errInfoChan, *forUser)
		gotErrInfo := <-errInfoChan
		if gotErrInfo.Err != nil {
			printCLI(gotErrInfo.Err)
			os.Exit(1)
		}
	}

	remembering := false
	if *drugargdose != 0 && *drugname == "none" && *changeLog == false {
		userLogsErrChan := make(chan drugdose.UserLogsError)
		go gotsetcfg.RecallDosing(db, ctx, userLogsErrChan, *forUser)
		gotUserLogsErr := <-userLogsErrChan
		err := gotUserLogsErr.Err
		if err != nil {
			printCLI("Couldn't recall dosing configuration: ", err)
			os.Exit(1)
		} else if err == nil && gotUserLogsErr.UserLogs != nil {
			remCfg := gotUserLogsErr.UserLogs[0]
			printCLI("Remembering from config using ID:", remCfg.StartTime)
			*forUser = remCfg.Username
			*drugname = remCfg.DrugName
			*drugroute = remCfg.DrugRoute
			*drugunits = remCfg.DoseUnits
			remembering = true
		}
	}

	if *removeInfoDrug != "none" {
		execCount++
		go gotsetcfg.RemoveSingleDrugInfo(db, ctx, errInfoChanHandled, *removeInfoDrug, *forUser)
	}

	remAmount := 0
	revRem := false
	if *removeOld != 0 {
		remAmount = *removeOld
	}

	if *removeNew != 0 {
		remAmount = *removeNew
		revRem = true
	}

	if *cleanLogs || remAmount != 0 {
		execCount++
		go gotsetcfg.RemoveLogs(db, ctx, errInfoChanHandled, *forUser,
			remAmount, revRem, *forID, *searchStr, getExact)
	}

	inputDose := false
	if *changeLog == false && remembering == false && *getLogs == false &&
		*dontLog == false && *searchExact == false {

		if *drugname != "none" ||
			*drugroute != "none" ||
			*drugargdose != 0 ||
			*drugunits != "none" {

			if *drugname == "none" {
				printCLI("No drug name specified, checkout: gopsydose -help")
			}

			if *drugroute == "none" {
				printCLI("No route specified, checkout: gopsydose -help")
			}

			if *drugargdose == 0 {
				printCLI("No dose specified, checkout: gopsydose -help")
			}

			if *drugunits == "none" {
				printCLI("No units specified, checkout: gopsydose -help")
			}

			if *drugname != "none" && *drugroute != "none" &&
				*drugargdose != 0 && *drugunits != "none" {

				inputDose = true
			}
		}
	}

	if remembering == true {
		inputDose = true
	}

	if inputDose == true || *dontLog == true && *drugname != "none" {
		err, cli := gotsetcfg.InitGraphqlClient()
		fetchErr := false
		if err == nil {
			errInfoChan := make(chan drugdose.ErrorInfo)
			go gotsetcfg.FetchFromSource(db, ctx, errInfoChan, *drugname, cli, *forUser)
			gotErrInfo := <-errInfoChan
			if gotErrInfo.Err != nil {
				fetchErr = true
				printCLI(gotErrInfo.Err)
			}

		} else {
			printCLI(err)
		}

		if *dontLog == false && fetchErr == false {
			synct := drugdose.SyncTimestamps{}
			execCount++
			go gotsetcfg.AddToDoseTable(db, ctx, errInfoChanHandled, &synct, *forUser, *drugname, *drugroute,
				float32(*drugargdose), *drugunits, float32(*drugperc), float32(*drugcost), *costCur,
				true)
		} else if *dontLog == true {
			err, convOutput, convUnit := gotsetcfg.ConvertUnits(db, ctx, *drugname,
				float32(*drugargdose), float32(*drugperc))
			if err != nil {
				printCLI(err)
				os.Exit(1)
			} else {
				convSubs := gotsetcfg.MatchAndReplace(db, ctx, *drugname, "substance")
				printCLI(fmt.Sprintf("Didn't log, converted dose: "+
					"%g ; units: %q ; substance: %q ; username: %q",
					convOutput, convUnit, convSubs, *forUser))
			}
		}
	}

	if *dontLog == false && *remember == true {
		execCount++
		go gotsetcfg.RememberDosing(db, ctx, errInfoChanHandled, *forUser, *forID)
	}

	if *changeLog {
		execCount++
		go gotsetcfg.ChangeUserLog(db, ctx, errInfoChanHandled, setType, *forID, *forUser, setValue)
	}

	errInfoChanHandled <- drugdose.ErrorInfo{}
	wg.Wait()
	// All functions which modify data have finished.

	userLogsErrChan := make(chan drugdose.UserLogsError)
	var gettingLogs bool = false
	var logsLimit bool = false

	if *getLogs {
		if *noGetLimit {
			go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, 0, *forID,
				*forUser, false, *searchStr, getExact)
		} else {
			go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, 100, *forID,
				*forUser, false, *searchStr, getExact)
			logsLimit = true
		}
		gettingLogs = true
	} else if *getNewLogs != 0 {
		go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, *getNewLogs, 0,
			*forUser, true, *searchStr, getExact)
		gettingLogs = true
	} else if *getOldLogs != 0 {
		go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, *getOldLogs, 0,
			*forUser, false, *searchStr, getExact)
		gettingLogs = true
	}

	if gettingLogs == true {
		gotUserLogsErr := <-userLogsErrChan
		retLogs := gotUserLogsErr.UserLogs
		gotErr := gotUserLogsErr.Err
		if logsLimit == true {
			if retLogs != nil && len(retLogs) == 100 {
				printCLI("By default there is a limit of retrieving " +
					"and printing a maximum of 100 entries. " +
					"To avoid it, use the -no-get-limit option.")
			}
		}

		if gotErr != nil {
			printCLI(gotErr)
		} else {
			gotsetcfg.PrintLogs(retLogs, false)
		}
	}

	if *getLogsCount {
		logCountErrChan := make(chan drugdose.LogCountError)
		go gotsetcfg.GetLogsCount(db, ctx, *forUser, logCountErrChan)
		gotLogCountErr := <-logCountErrChan
		err := gotLogCountErr.Err
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
		printCLI("Total number of logs:", gotLogCountErr.LogCount, "; for user:", gotLogCountErr.Username)
	}

	if *getTotalCosts {
		costsErrChan := make(chan drugdose.CostsError)
		go gotsetcfg.GetTotalCosts(db, ctx, costsErrChan, *forUser)
		gotCostsErr := <-costsErrChan
		err := gotCostsErr.Err
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
		drugdose.PrintTotalCosts(gotCostsErr.Costs, false)
	}

	if *getTimes {
		timeTillErrChan := make(chan drugdose.TimeTillError)
		go gotsetcfg.GetTimes(db, ctx, timeTillErrChan, *forUser, *forID)
		gotTimeTillErr := <-timeTillErrChan
		err := gotTimeTillErr.Err
		if err != nil {
			printCLI("Times couldn't be retrieved because of an error:", err)
			os.Exit(1)
		} else {
			err = gotsetcfg.PrintTimeTill(gotTimeTillErr, false)
			if err != nil {
				printCLI("Couldn't print times because of an error:", err)
				os.Exit(1)
			}
		}
	}

	if *getUsers {
		allUsersErrChan := make(chan drugdose.AllUsersError)
		go gotsetcfg.GetUsers(db, ctx, allUsersErrChan, *forUser)
		gotAllUsersErr := <-allUsersErrChan
		err = gotAllUsersErr.Err
		ret := gotAllUsersErr.AllUsers
		if err != nil {
			printCLI("Couldn't get users because of an error:", err)
			os.Exit(1)
		} else if err == nil {
			str := fmt.Sprint("All users: ")
			for i := 0; i < len(ret); i++ {
				str += fmt.Sprintf("%q ; ", ret[i])
			}
			printCLI(str)
		}
	}

	getNamesWhich := "none"
	getNamesValue := ""
	if *getSubNames != "" {
		getNamesWhich = "substance"
		getNamesValue = *getSubNames
	} else if *getRouteNames != "" {
		getNamesWhich = "route"
		getNamesValue = *getRouteNames
	} else if *getUnitsNames != "" {
		getNamesWhich = "units"
		getNamesValue = *getUnitsNames
	}

	if getNamesWhich != "none" {
		drugNamesErrChan := make(chan drugdose.DrugNamesError)
		go gotsetcfg.GetAllAltNames(db, ctx, drugNamesErrChan,
			getNamesValue, getNamesWhich, false, *forUser)
		gotDrugNamesErr := <-drugNamesErrChan
		subsNames := gotDrugNamesErr.DrugNames
		err = gotDrugNamesErr.Err
		if err != nil {
			printCLI("Couldn't get substance names, because of error:", err)
			os.Exit(1)
		} else {
			fmt.Print("For " + getNamesWhich + ": " + getNamesValue + " ; Alternative names: ")
			for i := 0; i < len(subsNames); i++ {
				fmt.Print(subsNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	getUniqueNames := false
	getInfoNames := false
	useCol := ""
	if *getLocalInfoDrugs {
		getUniqueNames = true
		getInfoNames = true
		useCol = drugdose.InfoDrugNameCol
	} else if *getLoggedDrugs {
		getUniqueNames = true
		useCol = drugdose.LogDrugNameCol
	}

	if getUniqueNames == true {
		drugNamesErrChan := make(chan drugdose.DrugNamesError)
		go gotsetcfg.GetLoggedNames(db, ctx, drugNamesErrChan, *&getInfoNames, *forUser, useCol)
		gotDrugNamesErr := <-drugNamesErrChan
		err = gotDrugNamesErr.Err
		locinfolist := gotDrugNamesErr.DrugNames
		if err != nil {
			printCLI("Error getting drug names list:", err)
			os.Exit(1)
		} else {
			if getInfoNames {
				str := fmt.Sprint("For source: " + gotsetcfg.UseSource + " ; All local drugs: ")
				for i := 0; i < len(locinfolist); i++ {
					str += fmt.Sprint(locinfolist[i] + " ; ")
				}
				printCLI(str)
			} else {
				str := fmt.Sprint("Logged unique drug names: ")
				for i := 0; i < len(locinfolist); i++ {
					str += fmt.Sprint(locinfolist[i] + " ; ")
				}
				printCLI(str)
			}
		}
	}

	if *getLocalInfoDrug != "none" {
		drugInfoErrChan := make(chan drugdose.DrugInfoError)
		go gotsetcfg.GetLocalInfo(db, ctx, drugInfoErrChan, *getLocalInfoDrug, *forUser)
		gotDrugInfoErr := <-drugInfoErrChan
		locinfo := gotDrugInfoErr.DrugI
		err = gotDrugInfoErr.Err
		if err != nil {
			printCLI("Couldn't get info for drug because of error:", err)
			os.Exit(1)
		} else {
			gotsetcfg.PrintLocalInfo(locinfo, false)
		}
	}

	if *getDBSize {
		ret := gotsetcfg.GetDBSize()
		retMiB := (ret / 1024) / 1024
		printCLI("Total DB size returned:", retMiB, "MiB ;", ret, "bytes")
	}
}
