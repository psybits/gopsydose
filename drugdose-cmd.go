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

	localInfoDrug = flag.String(
		"local-info-drug",
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

	dontLog = flag.Bool(
		"dont-log",
		false,
		"only fetch info about drug to local DB,\n"+
			"but don't log anything")

	removeDrug = flag.String(
		"remove-info-drug",
		"none",
		"remove all entries of a single drug from the info DB")

	cleanDB = flag.Bool(
		"clean-db",
		false,
		"remove all tables from the DB")

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

	printSettings = flag.Bool(
		"print-settings",
		false,
		"print the settings")

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

	setcfg := drugdose.InitSettingsStruct(
		drugdose.DefaultMaxLogsPerUser,
		*sourcecfg,
		drugdose.DefaultAutoFetch,
		*dbDir,
		drugdose.DefaultDBName,
		drugdose.DefaultAutoRemove,
		drugdose.DefaultDBDriver,
		drugdose.DefaultMySQLAccess,
		drugdose.DefaultVerbose,
		drugdose.DefaultTimezone,
	)
	if setcfg == nil {
		fmt.Println("Settings struct not initialised.")
		os.Exit(1)
	}

	ret := setcfg.InitSettings(*recreateSettings)
	if !ret {
		drugdose.VerbosePrint("Settings weren't initialised.", *verbose)
	}

	gotsetcfg := drugdose.GetSettings(*printSettings)

	gotsrc := drugdose.InitSourceStruct(gotsetcfg.UseSource, *apiURL)
	ret = gotsetcfg.InitSourceSettings(gotsrc, *recreateSources)
	if !ret {
		drugdose.VerbosePrint("Sources file wasn't initialised.", *verbose)
	}

	gotsrcData := drugdose.GetSourceData()

	if *getDirs {
		fmt.Println("DB Dir:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
		fmt.Println("Settings Dir:", drugdose.InitSettingsDir())
	}

	if *forget {
		ret := gotsetcfg.ForgetConfig(*forUser)
		if !ret {
			fmt.Println("Couldn't forget remember config, because of an error.")
		}
	}

	remembering := false
	if *drugargdose != 0 && *drugname == "none" && *changeLog == false {
		got := gotsetcfg.GetUserSettings("useIDForRemember", *forUser)
		if got != "" && got != "9999999999" {
			remCfg := gotsetcfg.RememberConfig(*forUser)
			if remCfg != nil {
				fmt.Println("Remembering from config using ID:", got)
				*forUser = remCfg.Username
				*drugname = remCfg.DrugName
				*drugroute = remCfg.DrugRoute
				*drugunits = remCfg.DoseUnits
				remembering = true
			}
		}
	}

	if *stopOnCfgInit {
		fmt.Println("Stopping after config file initialization.")
		os.Exit(0)
	}

	if gotsetcfg.DBDriver == "sqlite3" {
		gotsetcfg.InitDBFileStructure()

		ret = gotsetcfg.InitAllDBTables()
		if !ret {
			fmt.Println("Database didn't get initialised, because of an error, exiting.")
			os.Exit(1)
		}
	} else if gotsetcfg.DBDriver == "mysql" {
		ret = gotsetcfg.InitAllDBTables()
		if !ret {
			fmt.Println("Database didn't get initialised, because of an error, exiting.")
			os.Exit(1)
		}
	} else {
		fmt.Println("No proper driver selected. Choose sqlite3 or mysql!")
		os.Exit(1)
	}

	if *dbDriverInfo {
		fmt.Println("Using database driver:", gotsetcfg.DBDriver)
		fmt.Println("Database path:", gotsetcfg.DBSettings[gotsetcfg.DBDriver].Path)
	}

	if *cleanDB {
		ret := gotsetcfg.CleanDB()
		if !ret {
			fmt.Println("Database couldn't be cleaned, because of an error.")
		}
	}

	if *stopOnDbInit {
		fmt.Println("Stopping after database initialization.")
		os.Exit(0)
	}

	if *removeDrug != "none" {
		ret := gotsetcfg.RemoveSingleDrugInfoDB(*removeDrug)
		if !ret {
			fmt.Println("Failed to remove single drug from info database:", *removeDrug)
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
			fmt.Println("Couldn't remove logs because of an error.")
		}
	}

	if *cleanNames {
		ret := gotsetcfg.CleanNames()
		if !ret {
			fmt.Println("Couldn't remove alt names from DB because of an error.")
		}
	}

	if *getDBSize {
		ret := gotsetcfg.GetDBSize()
		retMiB := (ret / 1024) / 1024
		fmt.Println("Total DB size returned:", retMiB, "MiB ;", ret, "bytes")
	}

	if *getUsers {
		ret := gotsetcfg.GetUsers()
		if len(ret) == 0 {
			fmt.Println("Couldn't get users because of an error.")
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
		fmt.Println("Total number of logs:", ret, "; for user:", *forUser)
	}

	if *getLogs {
		var ret []drugdose.UserLog
		if *noGetLimit {
			ret = gotsetcfg.GetLogs(0, *forID, *forUser, true, false, true, *searchStr)
		} else {
			ret = gotsetcfg.GetLogs(100, *forID, *forUser, false, false, true, *searchStr)
			if ret != nil && len(ret) == 100 {
				fmt.Println("By default there is a limit of retrieving " +
					"and printing a maximum of 100 entries. " +
					"To avoid it, use the -no-get-limit option.")
			}
		}
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	} else if *getNewLogs != 0 {
		ret := gotsetcfg.GetLogs(*getNewLogs, 0, *forUser, false, true, true, *searchStr)
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	} else if *getOldLogs != 0 {
		ret := gotsetcfg.GetLogs(*getOldLogs, 0, *forUser, false, false, true, *searchStr)
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	}

	if *getLocalInfoDrugs {
		locinfolist := gotsetcfg.GetLocalInfoNames()
		if len(locinfolist) == 0 {
			fmt.Println("Couldn't get database list of drugs names from info table.")
		} else {
			fmt.Print("All local drugs: ")
			for i := 0; i < len(locinfolist); i++ {
				fmt.Print(locinfolist[i] + " ; ")
			}
			fmt.Println()
		}
	}

	if *getSubNames != "" {
		subsNames := gotsetcfg.GetAllNames(*getSubNames, "substance", true)
		if subsNames == nil {
			fmt.Println("Couldn't get substance names, because of an error.")
		} else {
			fmt.Print("For substance: " + *getSubNames + " ; Alternative names: ")
			for i := 0; i < len(subsNames); i++ {
				fmt.Print(subsNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *getRouteNames != "" {
		routeNames := gotsetcfg.GetAllNames(*getRouteNames, "route", true)
		if routeNames == nil {
			fmt.Println("Couldn't get route names, because of an error.")
		} else {
			fmt.Print("For route: " + *getRouteNames + " ; Alternative names: ")
			for i := 0; i < len(routeNames); i++ {
				fmt.Print(routeNames[i] + ", ")
			}
			fmt.Println()
		}
	}

	if *localInfoDrug != "none" {
		locinfo := gotsetcfg.GetLocalInfo(*localInfoDrug, true)
		if len(locinfo) == 0 {
			fmt.Println("Couldn't get database info for drug:", *localInfoDrug)
		}
	}

	inputDose := false
	if *changeLog == false && remembering == false {
		if *drugname != "none" ||
			*drugroute != "none" ||
			*drugargdose != 0 ||
			*drugunits != "none" {

			if *drugname == "none" {
				fmt.Println("No drug name specified, checkout: gopsydose -help")
			}

			if *drugroute == "none" {
				fmt.Println("No route specified, checkout: gopsydose -help")
			}

			if *drugargdose == 0 {
				fmt.Println("No dose specified, checkout: gopsydose -help")
			}

			if *drugunits == "none" {
				fmt.Println("No units specified, checkout: gopsydose -help")
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

	if inputDose == true {
		drugdose.VerbosePrint("Using API from settings.toml: "+gotsetcfg.UseSource, *verbose)
		drugdose.VerbosePrint("Got API URL from sources.toml: "+gotsrcData[gotsetcfg.UseSource].API_URL, *verbose)

		cli := gotsetcfg.InitGraphqlClient()
		if cli != nil {
			if gotsetcfg.UseSource == "psychonautwiki" {
				ret := gotsetcfg.FetchPsyWiki(*drugname, *drugroute, cli, true)
				if !ret {
					fmt.Println("Didn't fetch anything.")
				}
			} else {
				fmt.Println("No valid API selected:", gotsetcfg.UseSource)
				os.Exit(1)
			}

			if *dontLog == false {
				ret := gotsetcfg.AddToDoseDB(*forUser, *drugname, *drugroute,
					float32(*drugargdose), *drugunits, float32(*drugperc), true)
				if !ret {
					fmt.Println("Dose wasn't logged.")
				}
			}
		} else {
			fmt.Println("Something went wrong when initialising the client," +
				"\nnothing was fetched or logged.")
		}
	}

	if *dontLog == false {
		if *remember {
			userSettingsForID := strconv.FormatInt(*forID, 10)
			ret = gotsetcfg.SetUserSettings("useIDForRemember", *forUser, userSettingsForID)
			if !ret {
				fmt.Println("Couldn't remember config, because of an error.")
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
			fmt.Println("Couldn't change user log, because of an error.")
		}
	}

	if *getTimes {
		times := gotsetcfg.GetTimes(*forUser, *forID, true)
		if times == nil {
			fmt.Println("Times couldn't be retrieved because of an error.")
		}
	}
}
