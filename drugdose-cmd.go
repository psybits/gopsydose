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

	apiURL = flag.String(
		"apiurl",
		drugdose.DefaultAPI,
		"the URL of the API that you want to initialise for\n"+
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
	fmt.Print("gopsydose: CLI: ")
	fmt.Println(str...)
}

// Same as printCLI(), but only for verbose output and is optional.
func printCLIVerbose(verbose bool, str ...any) {
	if verbose == true {
		printCLI(str...)
	}
}

func main() {
	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nCheckout the list above for more info!")
		fmt.Fprintln(os.Stderr, "\nAlso checkout the example.alias file in the git repo\nfor an easier to use environment!")
		fmt.Fprintln(os.Stderr, "\nTo delete the 3 oldest dosages: gopsydose -clean-old-logs 3")
		fmt.Fprintln(os.Stderr, "\nTo see last dose: gopsydose -get-last 1")
		fmt.Fprintln(os.Stderr, "To see last dose progression: gopsydose -get-times")
		fmt.Fprintln(os.Stderr, "\nExample:\ngopsydose -drug alcohol -route oral -dose 355 -units ml -perc 4.5")
		fmt.Fprintln(os.Stderr, "If not taken at once, when finished dosing: gopsydose -set-end-time")
		fmt.Fprintln(os.Stderr, "\nThe flag -set-end-time means, for example when finished"+
			"\ndrinking a glass of beer.")
		fmt.Fprintln(os.Stderr, "\nExample 2:\ngopsydose -drug lsd -route sublingual -dose 100 -units ug")
		fmt.Fprintln(os.Stderr, "\nThe second example shouldn't require the -set-end-time command,"+
			"\nsince it's usually taken all at once.")
	}

	flag.Parse()

	setcfg := drugdose.Config {
		MaxLogsPerUser:  drugdose.DefaultMaxLogsPerUser,
		UseSource:       *sourcecfg,
		AutoFetch:       drugdose.DefaultAutoFetch,
		AutoRemove:      drugdose.DefaultAutoRemove,
		DBDriver:        drugdose.DefaultDBDriver,
		VerbosePrinting: drugdose.DefaultVerbose,
		DBSettings:      nil,
		Timezone:        drugdose.DefaultTimezone,
	}

	ret := setcfg.InitDBSettings(*dbDir, drugdose.DefaultDBName, drugdose.DefaultMySQLAccess)
	if !ret {
		printCLI("DBSettings not initialised properly.")
		os.Exit(1)
	}	

	ret = setcfg.InitSettingsFile(*recreateSettings, *verbose)
	if !ret {
		printCLIVerbose(*verbose, "The settings file wasn't initialised.")
	}

	gotsetcfg := drugdose.GetSettings()	

	if *verbose == true {
		gotsetcfg.VerbosePrinting = true
	}

	gotsrc := map[string]drugdose.SourceConfig{
		gotsetcfg.UseSource: {
			API_ADDRESS: *apiURL,
		},
	}

	ret = gotsetcfg.InitSourceSettings(gotsrc, *recreateSources)
	if !ret {
		printCLIVerbose(*verbose, "The sources file wasn't initialised.")
	}

	gotsrcData := drugdose.GetSourceData()

	if *getDirs {
		printCLI("DB Dir:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
		printCLI("Settings Dir:", drugdose.InitSettingsDir())
	}

	if *forget {
		ret := gotsetcfg.ForgetConfig(*forUser)
		if !ret {
			printCLI("Couldn't 'forget' the remember config, because of an error.")
		}
	}

	remembering := false
	if *drugargdose != 0 && *drugname == "none" && *changeLog == false {
		got := gotsetcfg.GetUserSettings("useIDForRemember", *forUser)
		if got != "" && got != drugdose.ForgetInputConfigMagicNumber {
			remCfg := gotsetcfg.RememberConfig(*forUser)
			if remCfg != nil {
				printCLI("Remembering from config using ID:", got)
				*forUser = remCfg.Username
				*drugname = remCfg.DrugName
				*drugroute = remCfg.DrugRoute
				*drugunits = remCfg.DoseUnits
				remembering = true
			}
		}
	}

	if *stopOnCfgInit {
		printCLI("Stopping after config file initialization.")
		os.Exit(0)
	}

	if *cleanDB == false {
		if gotsetcfg.DBDriver == "sqlite3" {
			gotsetcfg.InitDBFileStructure()

			ret = gotsetcfg.InitAllDBTables()
			if !ret {
				printCLI("Database didn't get initialised, because of an error, exiting.")
				os.Exit(1)
			}
		} else if gotsetcfg.DBDriver == "mysql" {
			ret = gotsetcfg.InitAllDBTables()
			if !ret {
				printCLI("Database didn't get initialised, because of an error, exiting.")
				os.Exit(1)
			}
		} else {
			printCLI("No proper driver selected. Choose sqlite3 or mysql!")
			os.Exit(1)
		}
	}

	if *dbDriverInfo {
		printCLI("Using database driver:", gotsetcfg.DBDriver)
		printCLI("Database path:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
	}

	if *cleanInfo {
		ret := gotsetcfg.CleanInfo()
		if !ret {
			printCLI("Info table: " + gotsetcfg.UseSource + "couldn't be removed because of an error.")
		}
	}

	if *cleanDB {
		ret := gotsetcfg.CleanDB()
		if !ret {
			printCLI("Database couldn't be cleaned, because of an error.")
		}
	}

	if *stopOnDbInit {
		printCLI("Stopping after database initialization.")
		os.Exit(0)
	}

	if *removeInfoDrug != "none" {
		ret := gotsetcfg.RemoveSingleDrugInfoDB(*removeInfoDrug)
		if !ret {
			printCLI("Failed to remove single drug from info database:", *removeInfoDrug)
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
		ret := gotsetcfg.RemoveLogs(*forUser, remAmount, revRem, *forID, *searchStr)
		if !ret {
			printCLI("Couldn't remove logs because of an error.")
		}
	}

	if *cleanNames {
		ret := gotsetcfg.CleanNames()
		if !ret {
			printCLI("Couldn't remove alt names from DB because of an error.")
		}
	}

	if *getDBSize {
		ret := gotsetcfg.GetDBSize()
		retMiB := (ret / 1024) / 1024
		printCLI("Total DB size returned:", retMiB, "MiB ;", ret, "bytes")
	}

	if *getUsers {
		ret := gotsetcfg.GetUsers()
		if len(ret) == 0 {
			printCLI("Couldn't get users because of an error.")
		} else {
			fmt.Print("All users: ")
			for i := 0; i < len(ret); i++ {
				fmt.Print(ret[i] + " ; ")
			}
			fmt.Println()
		}
	}

	if *getLogsCount {
		ret := gotsetcfg.GetLogsCount(*forUser)
		printCLI("Total number of logs:", ret, "; for user:", *forUser)
	}

	if *getLogs {
		var ret []drugdose.UserLog
		if *noGetLimit {
			ret = gotsetcfg.GetLogs(0, *forID, *forUser, true, false, true, *searchStr)
		} else {
			ret = gotsetcfg.GetLogs(100, *forID, *forUser, false, false, true, *searchStr)
			if ret != nil && len(ret) == 100 {
				printCLI("By default there is a limit of retrieving " +
					"and printing a maximum of 100 entries. " +
					"To avoid it, use the -no-get-limit option.")
			}
		}
		if ret == nil {
			printCLI("No logs could be returned.")
		}
	} else if *getNewLogs != 0 {
		ret := gotsetcfg.GetLogs(*getNewLogs, 0, *forUser, false, true, true, *searchStr)
		if ret == nil {
			printCLI("No logs could be returned.")
		}
	} else if *getOldLogs != 0 {
		ret := gotsetcfg.GetLogs(*getOldLogs, 0, *forUser, false, false, true, *searchStr)
		if ret == nil {
			printCLI("No logs could be returned.")
		}
	}

	if *getLocalInfoDrugs {
		locinfolist := gotsetcfg.GetLocalInfoNames()
		if len(locinfolist) == 0 {
			printCLI("Couldn't get database list of drugs names from info table.")
		} else {
			fmt.Print("For source: " + gotsetcfg.UseSource + " ; All local drugs: ")
			for i := 0; i < len(locinfolist); i++ {
				fmt.Print(locinfolist[i] + " ; ")
			}
			fmt.Println()
		}
	}

	if *getSubNames != "" {
		subsNames := gotsetcfg.GetAllNames(*getSubNames, "substance", false)
		if subsNames == nil {
			printCLI("Couldn't get substance names, because of an error.")
		} else {
			fmt.Print("For substance: " + *getSubNames + " ; Alternative names: ")
			for i := 0; i < len(subsNames); i++ {
				fmt.Print(subsNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *getRouteNames != "" {
		routeNames := gotsetcfg.GetAllNames(*getRouteNames, "route", false)
		if routeNames == nil {
			printCLI("Couldn't get route names, because of an error.")
		} else {
			fmt.Print("For route: " + *getRouteNames + " ; Alternative names: ")
			for i := 0; i < len(routeNames); i++ {
				fmt.Print(routeNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *getUnitsNames != "" {
		unitsNames := gotsetcfg.GetAllNames(*getUnitsNames, "units", false)
		if unitsNames == nil {
			printCLI("Couldn't get units names, because of an error.")
		} else {
			fmt.Print("For unit: " + *getUnitsNames + " ; Alternative names: ")
			for i := 0; i < len(unitsNames); i++ {
				fmt.Print(unitsNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *getLocalInfoDrug != "none" {
		locinfo := gotsetcfg.GetLocalInfo(*getLocalInfoDrug, true)
		if len(locinfo) == 0 {
			printCLI("Couldn't get database info for drug:", *getLocalInfoDrug)
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
		printCLIVerbose(*verbose, "Using API from settings.toml: "+gotsetcfg.UseSource)
		printCLIVerbose(*verbose, "Got API URL from sources.toml: "+gotsrcData[gotsetcfg.UseSource].API_ADDRESS)

		cli := gotsetcfg.InitGraphqlClient()
		if cli != nil {
			if gotsetcfg.UseSource == "psychonautwiki" {
				ret := gotsetcfg.FetchPsyWiki(*drugname, cli)
				if !ret {
					printCLIVerbose(*verbose, "Didn't fetch anything from:", gotsetcfg.UseSource)
				}
			} else {
				printCLI("No valid API selected:", gotsetcfg.UseSource)
				os.Exit(1)
			}

			if *dontLog == false {
				ret := gotsetcfg.AddToDoseDB(*forUser, *drugname, *drugroute,
					float32(*drugargdose), *drugunits, float32(*drugperc), true)
				if !ret {
					printCLI("Dose wasn't logged.")
				}
			}
		} else {
			printCLI("Something went wrong when initialising the client," +
				"\nnothing was fetched or logged.")
		}
	}

	if *dontLog == false {
		if *remember {
			userSettingsForID := strconv.FormatInt(*forID, 10)
			ret = gotsetcfg.SetUserSettings("useIDForRemember", *forUser, userSettingsForID)
			if !ret {
				printCLI("Couldn't remember config, because of an error.")
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

		ret := gotsetcfg.SetUserLogs(setType, *forID, *forUser, setValue)
		if !ret {
			printCLI("Couldn't change user log, because of an error.")
		}
	}

	if *getTimes {
		times := gotsetcfg.GetTimes(*forUser, *forID, true)
		if times == nil {
			printCLI("Times couldn't be retrieved because of an error.")
		}
	}
}
