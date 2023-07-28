package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

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
		"get all cached drugs names (from info tables, not logs)\n"+
			"according to set source")

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

	ctx, ctx_cancel, err := gotsetcfg.UseConfigTimeout()
	if err != nil {
		printCLI(err)
		os.Exit(1)
	}
	defer ctx_cancel()

	gotsetcfg.InitAllDB(ctx)

	db := gotsetcfg.OpenDBConnection(ctx)
	defer db.Close()
	///////////////////////////////////////////////////////////////////////

	errChannel := make(chan error)

	if *getDirs {
		printCLI("DB Dir:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
		err, gotsetdir := drugdose.InitSettingsDir()
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
		printCLI("Settings Dir:", gotsetdir)
	}

	if *overwriteNames {
		gotsetcfg.MatchName(db, ctx, "asd", "substance", false, true)
	}

	if *forget {
		go gotsetcfg.ForgetDosing(db, ctx, errChannel, *forUser)
		err := <-errChannel
		if err != nil {
			printCLI("Couldn't 'forget' the remember config, because of an error:", err)
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

	if *stopOnCfgInit {
		printCLI("Stopping after config file initialization.")
		os.Exit(0)
	}

	if *dbDriverInfo {
		printCLI("Using database driver:", gotsetcfg.DBDriver)
		printCLI("Database path:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
	}

	if *cleanInfo {
		err := gotsetcfg.CleanInfo(db, ctx)
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
	}

	if *cleanDB {
		err := gotsetcfg.CleanDB(db, ctx)
		if err != nil {
			printCLI(err)
			os.Exit(1)
		}
	}

	if *stopOnDbInit {
		printCLI("Stopping after database initialization.")
		os.Exit(0)
	}

	if *removeInfoDrug != "none" {
		go gotsetcfg.RemoveSingleDrugInfoDB(db, ctx, errChannel, *removeInfoDrug)
		err := <-errChannel
		if err != nil {
			printCLI(err)
		}
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
		go gotsetcfg.RemoveLogs(db, ctx, errChannel, *forUser, remAmount, revRem, *forID, *searchStr)
		gotErr := <-errChannel
		if gotErr != nil {
			printCLI(gotErr)
		}
	}

	if *cleanNames {
		err := gotsetcfg.CleanNames(db, ctx, false)
		if err != nil {
			printCLI(err)
		}
	}

	if *getDBSize {
		ret := gotsetcfg.GetDBSize()
		retMiB := (ret / 1024) / 1024
		printCLI("Total DB size returned:", retMiB, "MiB ;", ret, "bytes")
	}

	if *getUsers {
		err, ret := gotsetcfg.GetUsers(db, ctx)
		if err != nil {
			printCLI("Couldn't get users because of an error:", err)
		} else if len(ret) == 0 {
			printCLI("No users logged.")
		} else {
			fmt.Print("All users: ")
			for i := 0; i < len(ret); i++ {
				fmt.Print(ret[i] + " ; ")
			}
			fmt.Println()
		}
	}

	if *getLogsCount {
		err, ret := gotsetcfg.GetLogsCount(db, ctx, *forUser)
		if err != nil {
			printCLI(err)
		}
		printCLI("Total number of logs:", ret, "; for user:", *forUser)
	}

	userLogsErrChan := make(chan drugdose.UserLogsError)
	var gettingLogs bool = false
	var logsLimit bool = false

	if *getLogs {
		if *noGetLimit {
			go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, 0, *forID, *forUser, false, *searchStr)
		} else {
			go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, 100, *forID, *forUser, false, *searchStr)
			logsLimit = true
		}
		gettingLogs = true
	} else if *getNewLogs != 0 {
		go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, *getNewLogs, 0, *forUser, true, *searchStr)
		gettingLogs = true
	} else if *getOldLogs != 0 {
		go gotsetcfg.GetLogs(db, ctx, userLogsErrChan, *getOldLogs, 0, *forUser, false, *searchStr)
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

	if *getLocalInfoDrugs {
		drugNamesErrChan := make(chan drugdose.DrugNamesError)
		go gotsetcfg.GetLocalInfoNames(db, ctx, drugNamesErrChan)
		gotDrugNamesErr := <-drugNamesErrChan
		err = gotDrugNamesErr.Err
		locinfolist := gotDrugNamesErr.DrugNames
		if err != nil {
			printCLI("Error getting drug names list:", err)
		} else if len(locinfolist) == 0 {
			printCLI("Empty list of drug names from info table.")
		} else {
			fmt.Print("For source: " + gotsetcfg.UseSource + " ; All local drugs: ")
			for i := 0; i < len(locinfolist); i++ {
				fmt.Print(locinfolist[i] + " ; ")
			}
			fmt.Println()
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
		go gotsetcfg.GetAllNames(db, ctx, drugNamesErrChan,
			getNamesValue, getNamesWhich, false)
		gotDrugNamesErr := <-drugNamesErrChan
		subsNames := gotDrugNamesErr.DrugNames
		err = gotDrugNamesErr.Err
		if err != nil {
			printCLI("Couldn't get substance names, because of error:", err)
		} else if len(subsNames) == 0 {
			printCLI("No names returned for " + getNamesWhich + ": " + getNamesValue)
		} else {
			fmt.Print("For " + getNamesWhich + ": " + getNamesValue + " ; Alternative names: ")
			for i := 0; i < len(subsNames); i++ {
				fmt.Print(subsNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *getLocalInfoDrug != "none" {
		drugInfoErrChan := make(chan drugdose.DrugInfoError)
		go gotsetcfg.GetLocalInfo(db, ctx, drugInfoErrChan, *getLocalInfoDrug)
		gotDrugInfoErr := <-drugInfoErrChan
		locinfo := gotDrugInfoErr.DrugI
		err = gotDrugInfoErr.Err
		if err != nil {
			printCLI("Couldn't get info for drug because of error:", err)
		} else if len(locinfo) == 0 {
			printCLI("No info returned for drug:", *getLocalInfoDrug)
		} else {
			gotsetcfg.PrintLocalInfo(locinfo, false)
		}
	}

	inputDose := false
	if *changeLog == false && remembering == false && *dontLog == false {
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
		if err == nil {
			go gotsetcfg.FetchFromSource(db, ctx, errChannel, *drugname, cli)
			err = <-errChannel
			if err != nil {
				printCLI(err)
				*dontLog = true
			}
		} else {
			printCLI(err)
		}

		synct := drugdose.SyncTimestamps{}
		if *dontLog == false {
			go gotsetcfg.AddToDoseDB(db, ctx, errChannel, &synct, *forUser, *drugname, *drugroute,
				float32(*drugargdose), *drugunits, float32(*drugperc), true)
			gotErr := <-errChannel
			if gotErr != nil {
				printCLI(gotErr)
			}
		}
	}

	if *dontLog == false {
		if *remember {
			go gotsetcfg.RememberDosing(db, ctx, errChannel, *forUser, *forID)
			err = <-errChannel
			if err != nil {
				printCLI(err)
			}
		}
	}

	if *changeLog {
		setType := ""
		setValue := ""
		if *startTime != "none" {
			setType = "start-time"
			setValue = *startTime
		} else if *endTime != "none" {
			setType = "end-time"
			setValue = *endTime
		} else if *drugname != "none" {
			setType = "drug"
			setValue = *drugname
		} else if *drugargdose != 0 {
			setType = "dose"
			setValue = strconv.FormatFloat(*drugargdose, 'f', -1, 64)
		} else if *drugunits != "none" {
			setType = "units"
			setValue = *drugunits
		} else if *drugroute != "none" {
			setType = "route"
			setValue = *drugroute
		}

		go gotsetcfg.SetUserLogs(db, ctx, errChannel, setType, *forID, *forUser, setValue)
		err = <-errChannel
		if err != nil {
			printCLI(err)
		}
	}

	if *getTimes {
		timeTillErrChan := make(chan drugdose.TimeTillError)
		go gotsetcfg.GetTimes(db, ctx, timeTillErrChan, *forUser, *forID)
		gotTimeTillErr := <-timeTillErrChan
		err := gotTimeTillErr.Err
		if err != nil {
			printCLI("Times couldn't be retrieved because of an error:", err)
		} else {
			err = gotsetcfg.PrintTimeTill(gotTimeTillErr, false)
			if err != nil {
				printCLI("Couldn't print times because of an error:", err)
			}
		}
	}
}
