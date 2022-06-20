package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
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
		"oral, smoked, sublingual, insufflation, inhalation,\nintravenous, etc.")

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
		"this is only used for alcohol currently, again just a number,\nno % around it")

	setTime = flag.Bool(
		"set-time",
		false,
		"set the current time as the time on the last log,\n"+
			"default is end time,\n"+
			"choose -start-time if you wish to set the starting time of the dosage\n"+
			"don't forget to checkout -set-custom-time and -for-id as well")

	startTime = flag.Bool(
		"start-time",
		false,
		"make changes to start time of dosage, instead of default end time")

	setCustomTime = flag.Int64(
		"set-custom-time",
		0,
		"set the time in unix seconds, for last log\n"+
			"or if you add -for-id, for a particular log\n"+
			"this is in case you forgot to set it in time")

	forUser = flag.String(
		"user",
		"default",
		"log for a specific user, for example if you're looking\nafter a friend")

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

	forID = flag.Int64(
		"for-id",
		0,
		"perform and action for a particular id\n"+
			"current works for:\n"+
			"-get-logs -set-time -get-times\n"+
			"-clean-new-logs -clean-old-logs")

	apiName = flag.String(
		"apiname",
		"default",
		"the name of the API that you want to initialise for\nsettings.toml and sources.toml")

	apiURL = flag.String(
		"apiurl",
		"default",
		"the URL of the API that you want to initialise for\nsources.toml combined with -apiname")

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
		"default",
		"full path of the DB directory, this will work only on"+
			"\nthe initial run, you can change it later in the settings.toml,"+
			"\ndon't forget to delete the old DB directory")

	getDirs = flag.Bool(
		"get-dirs",
		false,
		"prints the settings and DB directories path")

	localInfoDrug = flag.String(
		"local-info-drug",
		"none",
		"print info about drug from local DB,\n"+
			"for example if you've forgotten routes and units")

	dontLog = flag.Bool(
		"dont-log",
		false,
		"only fetch info about drug to local DB, but don't log anything")

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
			"can be combined with -for-id to get times for a specific ID\n"+
			"use -get-logs, to see the IDs")

	stopOnCfgInit = flag.Bool(
		"stop-on-config-init",
		false,
		"stops the program once the config\nfiles have been initialised")

	stopOnDbInit = flag.Bool(
		"stop-on-db-init",
		false,
		"stops the program once the DB file has\nbeen created and initialised, if it doesn't exists already")

	verbose = flag.Bool(
		"verbose",
		false,
		"print extra information")

	remember = flag.Bool(
		"remember",
		false,
		"remember last dose config")

	forget = flag.Bool(
		"forget",
		false,
		"forget the last set remember config")

	userDBDriver = flag.String(
		"DBDriver",
		"configfile",
		"use mysql or sqlite3 as DB driver")

	mysqlAccess = flag.String(
		"MySQLAccess",
		"none",
		"user:password@tcp(127.0.0.1:3306)/database")

	dbDriverInfo = flag.Bool(
		"DBDriverInfo",
		false,
		"show info about the current configured database driver")

	printSettings = flag.Bool(
		"print-settings",
		false,
		"print the settings")

	printSources = flag.Bool(
		"print-sources",
		false,
		"print the sources config file")
)

type rememberConfig struct {
	User  string
	Drug  string
	Route string
	Units string
	Perc  float64
	API   string
}

// TODO: eventually make an exported function in drugdose/
// For that, the config needs to be stored in the DB per user.
func rememberDoseConfig(user string, drug string, route string,
	units string, perc float64, api string, path string) bool {

	newConfig := rememberConfig{
		User:  user,
		Drug:  drug,
		Route: route,
		Units: units,
		Perc:  perc,
		API:   api,
	}

	b, err := toml.Marshal(newConfig)
	if err != nil {
		fmt.Println(err)
		return false
	}

	path = path + "/remember.toml"
	fmt.Println("Writing to remember file:", path)
	file, err := os.Create(path)
	if err != nil {
		fmt.Println(err)
		return false
	}

	err = file.Chmod(0600)
	if err != nil {
		fmt.Println(err)
		return false
	}

	_, err = file.WriteString(string(b))
	if err != nil {
		fmt.Println(err)
		return false
	}

	err = file.Close()
	if err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

// TODO: eventually make an exported function in drugdose/
func readRememberConfig(path string) *rememberConfig {
	path = path + "/remember.toml"
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	file, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if len(file) == 0 {
		fmt.Println("Config is empty.")
		return nil
	}

	cfg := &rememberConfig{}

	err = toml.Unmarshal(file, cfg)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return cfg
}

// TODO: eventually make an exported function in drugdose/
func forgetConfig(path string) bool {
	gotCfg := readRememberConfig(path)
	if gotCfg == nil {
		fmt.Println("Problem with reading remember config.")
		return false
	}

	path = path + "/remember.toml"

	os.Remove(path)

	fmt.Println("Removed file:", path)
	fmt.Println("Forgot config.")

	return true
}

func main() {
	flag.Usage = func() {
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nExample\ngopsydose -drug alcohol -route oral -dose 355 -units ml -perc 4.5")
		fmt.Fprintln(os.Stderr, "If not taken at once, when finished dosing: gopsydose -set-end-time")
		fmt.Fprintln(os.Stderr, "\n-set-end-time means, for example when finished"+
			"\ndrinking a glass of beer.")
		fmt.Fprintln(os.Stderr, "\nExample 2: gopsydose -drug lsd -route sublingual -dose 100 -units ug")
		fmt.Fprintln(os.Stderr, "The second example shouldn't require the -set-end-time command,"+
			"\nsince it's usually taken all at once.")
		fmt.Fprintln(os.Stderr, "\nTo see last dose: gopsydose -get-last 1")
		fmt.Fprintln(os.Stderr, "To see last dose progression: gopsydose -get-times")
		fmt.Fprintln(os.Stderr, "\nTo delete the 3 oldest dosages: gopsydose -clean-old-logs 3")
		fmt.Fprintln(os.Stderr, "\nCheckout the list above for more info!")
	}

	flag.Parse()

	setcfg := drugdose.InitSettingsStruct(100, *apiName, true, *dbDir, false)
	if setcfg == nil {
		fmt.Println("Settings struct not initialised.")
		os.Exit(1)
	}

	ret := setcfg.InitSettings(*recreateSettings, *verbose)
	if !ret {
		drugdose.VerbosePrint("Settings weren't initialised.", *verbose)
	}

	gotsetcfg := drugdose.GetSettings(*printSettings)

	if *userDBDriver == "configfile" {
		*userDBDriver = gotsetcfg.DBDriver
		*mysqlAccess = gotsetcfg.MySQLAccess
	}

	dbDriver := *userDBDriver

	gotsrc := drugdose.InitSourceStruct(gotsetcfg.UseAPI, *apiURL)
	ret = drugdose.InitSourceSettings(gotsrc, *recreateSources, *verbose)
	if !ret {
		drugdose.VerbosePrint("Sources file wasn't initialised.", *verbose)
	}

	gotsrcData := drugdose.GetSourceData(*printSources)

	if *getDirs {
		fmt.Println("DB Dir:", gotsetcfg.DBDir)
		fmt.Println("Settings Dir:", drugdose.InitSettingsDir())
	}

	if *forget {
		ret := forgetConfig(drugdose.InitSettingsDir())
		if !ret {
			fmt.Println("Couldn't forget remember config, because of an error.")
		}
	}

	if *drugargdose != 0 && *drugname == "none" {
		remCfg := readRememberConfig(drugdose.InitSettingsDir())
		if remCfg != nil {
			fmt.Println("Remembering from config.")
			*forUser = remCfg.User
			*drugname = remCfg.Drug
			*drugroute = remCfg.Route
			*drugunits = remCfg.Units
			*drugperc = remCfg.Perc
			*apiName = remCfg.API
		}
	}

	if *stopOnCfgInit {
		fmt.Println("Stopping after config file initialization.")
		os.Exit(0)
	}

	var path string
	if dbDriver == "sqlite3" {
		path = drugdose.CheckDBFileStruct(gotsetcfg.DBDir, "default", *verbose)
		ret = false

		if path != "" {
			ret = drugdose.CheckDBTables(dbDriver, path)
		}

		if path == "" || !ret {
			if path == "" {
				path = drugdose.InitFileStructure(gotsetcfg.DBDir, "default")
			}
			ret = drugdose.InitDrugDB(gotsetcfg.UseAPI, dbDriver, path)
			if !ret {
				fmt.Println("Database didn't get initialised, because of an error, exiting.")
				os.Exit(1)
			}
		}
	} else if dbDriver == "mysql" {
		path = *mysqlAccess
		ret = drugdose.CheckDBTables(dbDriver, path)
		if !ret {
			ret = drugdose.InitDrugDB(gotsetcfg.UseAPI, dbDriver, path)
			if !ret {
				fmt.Println("Database didn't get initialised, because of an error, exiting.")
				os.Exit(1)
			}
		}
	} else {
		fmt.Println("No proper driver selected. Choose sqlite3 or mysql!")
		os.Exit(1)
	}

	if *dbDriverInfo {
		fmt.Println("Using database driver:", dbDriver)
		fmt.Println("Database path:", path)
	}

	if *cleanDB {
		ret := drugdose.CleanDB(dbDriver, path)
		if !ret {
			fmt.Println("Database couldn't be cleaned, because of an error.")
		}
	}

	if *stopOnDbInit {
		fmt.Println("Stopping after database initialization.")
		os.Exit(0)
	}

	if *removeDrug != "none" {
		ret := drugdose.RemoveSingleDrugInfoDB(gotsetcfg.UseAPI, *removeDrug, dbDriver, path)
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
		ret := drugdose.RemoveLogs(dbDriver, path, *forUser, remAmount, revRem, *forID)
		if !ret {
			fmt.Println("Couldn't remove logs because of an error.")
		}
	}

	if *getLogs {
		ret := drugdose.GetLogs(0, *forID, *forUser, true, dbDriver, path, false, true)
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	} else if *getNewLogs != 0 {
		ret := drugdose.GetLogs(*getNewLogs, 0, *forUser, false, dbDriver, path, true, true)
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	} else if *getOldLogs != 0 {
		ret := drugdose.GetLogs(*getOldLogs, 0, *forUser, false, dbDriver, path, false, true)
		if ret == nil {
			fmt.Println("No logs could be returned.")
		}
	}

	if *localInfoDrug != "none" {
		locinfo := drugdose.GetLocalInfo(*localInfoDrug, gotsetcfg.UseAPI, dbDriver, path, true)
		if len(locinfo) == 0 {
			fmt.Println("Couldn't get database info for drug:", *localInfoDrug)
		}
	}

	if *setTime || *setCustomTime != 0 {
		var timeType bool
		timeType = true
		if *startTime {
			timeType = false
		}

		ret := drugdose.SetTime(dbDriver, path, *forUser, *forID, *setCustomTime, timeType)
		if !ret {
			fmt.Println("Couldn't set time, because of an error.")
		}
	}

	if *drugname != "none" ||
		*drugroute != "none" ||
		*drugargdose != 0 ||
		*drugunits != "none" ||
		*dontLog {

		if *drugname == "none" {
			fmt.Println("No drug name specified, checkout: gopsydose -help")
			os.Exit(1)
		}

		if *drugroute == "none" {
			fmt.Println("No route specified, checkout: gopsydose -help")
			os.Exit(1)
		}

		if *drugargdose == 0 {
			fmt.Println("No dose specified, checkout: gopsydose -help")
			os.Exit(1)
		}

		if *drugunits == "none" {
			fmt.Println("No units specified, checkout: gopsydose -help")
			os.Exit(1)
		}

		drugdose.VerbosePrint("Using API from settings.toml: "+gotsetcfg.UseAPI, *verbose)
		drugdose.VerbosePrint("Got API URL from sources.toml: "+gotsrcData[gotsetcfg.UseAPI].APIURL, *verbose)

		cli := gotsetcfg.InitGraphqlClient(gotsrcData[gotsetcfg.UseAPI].APIURL)
		if cli != nil {
			if gotsetcfg.UseAPI == "psychonautwiki" {
				ret := gotsetcfg.FetchPsyWiki(*drugname, *drugroute, cli, dbDriver, path)
				if !ret {
					fmt.Println("Didn't fetch anything.")
				}
			} else {
				fmt.Println("No valid API selected:", gotsetcfg.UseAPI)
				os.Exit(1)
			}

			if *remember {
				ret = rememberDoseConfig(*forUser, *drugname, *drugroute,
					*drugunits, *drugperc, *apiName, drugdose.InitSettingsDir())
				if !ret {
					fmt.Println("Couldn't remember config, because of an error.")
				}
			}

			if !*dontLog {
				ret := gotsetcfg.AddToDoseDB(*forUser, *drugname, *drugroute,
					float32(*drugargdose), *drugunits, float32(*drugperc),
					dbDriver, path, *apiName)
				if !ret {
					fmt.Println("Dose wasn't logged.")
				}
			}
		} else {
			fmt.Println("Something went wrong when initialising the client," +
				"\nnothing was fetched or logged.")
		}
	}

	if *getTimes {
		times := drugdose.GetTimes(dbDriver, path, *forUser, gotsetcfg.UseAPI, *forID, true)
		if times == nil {
			fmt.Println("Times couldn't be retrieved because of an error.")
		}
	}
}
