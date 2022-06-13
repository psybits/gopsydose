package drugdose

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/powerjungle/goalconvert/alconvert"
)

const db_dir = "GPD"
const db_name = "gpd.db"
const default_source = "psychonautwiki"
const default_username = "defaultUser"

// Encryption should be done by default unless specified not to by the user from the settings
// But first the official implementation for encryption has to be done in the sqlite module

func exitProgram() {
	fmt.Println("Exiting")
	os.Exit(1)
}

func errorCantCloseDB(filePath string, err error) {
	fmt.Println("Can't close DB file:", filePath+":", err)
	exitProgram()
}

func errorCantCreateDB(filePath string, err error) {
	fmt.Println("Error creating drug info DB file:", filePath+":", err)
	exitProgram()
}

func errorCantOpenDB(filePath string, err error) {
	fmt.Println("Error opening DB:", filePath+":", err)
	exitProgram()
}

type UserLog struct {
	StartTime int64
	Username  string
	EndTime   int64
	DrugName  string
	Dose      float32
	DoseUnits string
	DrugRoute string
}

type DrugInfo struct {
	DrugName      string
	DrugRoute     string
	Threshold     float32
	LowDoseMin    float32
	LowDoseMax    float32
	MediumDoseMin float32
	MediumDoseMax float32
	HighDoseMin   float32
	HighDoseMax   float32
	DoseUnits     string
	OnsetMin      float32
	OnsetMax      float32
	OnsetUnits    string
	ComeUpMin     float32
	ComeUpMax     float32
	ComeUpUnits   string
	PeakMin       float32
	PeakMax       float32
	PeakUnits     string
	OffsetMin     float32
	OffsetMax     float32
	OffsetUnits   string
	TotalDurMin   float32
	TotalDurMax   float32
	TotalDurUnits string
	TimeOfFetch   int64
}

func xtrastmt(col string, logical string) string {
	return logical + " " + col + " = ?"
}

func checkIfExistsDB(col string, table string, driver string,
	path string, xtrastmt []string, values ...interface{}) bool {

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	stmtstr := "select " + col + " from " + table + " where " + col + " = ?"
	if xtrastmt != nil {
		for i := 0; i < len(xtrastmt); i++ {
			stmtstr = stmtstr + " " + xtrastmt[i]
		}
	}

	// NOTE: this doesn't cause an SQL injection, because we're not taking col and table from an user input.
	stmt, err := db.Prepare(stmtstr)
	if err != nil {
		fmt.Println("SQL error in prepare for check if exists:", err)
		return false
	}
	defer stmt.Close()
	var got string

	err = stmt.QueryRow(values...).Scan(&got)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		fmt.Println("checkIfExistsDB: received weird error:", err)
		return false
	}

	return true
}

// Creates the basic file structure for the database, this should be run only once
func InitFileStructure(dbdir string, dbname string) string {
	if dbdir == "default" {
		dbdir = db_dir
	}

	if dbname == "default" {
		dbname = db_name
	}

	err := os.Mkdir(dbdir, 0700)
	if err != nil {
		fmt.Println("Error creating directory for DB:", err)
		exitProgram()
	}

	db_file_locat := dbdir + "/" + dbname

	file, err := os.Create(db_file_locat)
	if err != nil {
		errorCantCreateDB(db_file_locat, err)
	}

	err = file.Close()
	if err != nil {
		errorCantCloseDB(db_file_locat, err)
	}

	fmt.Println("Initialised the file structure")

	return db_file_locat
}

// Returns true if the file structure is already created, false otherwise
// Checks whether the db directory and minimum amount of files exist with the proper names in it
func CheckDBFileStruct(dbdir string, dbname string, verbose bool) string {
	if dbname == "default" {
		dbname = db_name
	}

	db_file_locat := dbdir + "/" + dbname

	if _, err := os.Stat(db_file_locat); err == nil {
		VerbosePrint(db_file_locat+": Exists", verbose)
	} else if errors.Is(err, os.ErrNotExist) {
		fmt.Println(db_file_locat+": Doesn't seem to exist:", err)
		return ""
	} else {
		panic(err)
	}

	return db_file_locat
}

// Remove all entries of a single drug from the local info DB, instead of deleting the whole DB.
func RemoveSingleDrugInfoDB(source string, drug string, driver string, path string) bool {
	if source == "default" {
		source = default_source
	}

	drug = MatchDrugName(drug)

	ret := checkIfExistsDB("drugName", source, driver, path, nil, drug)
	if !ret {
		fmt.Println("No such drug in info database:", drug)
		return false
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare("delete from " + source +
		" where drugName = ?")
	if err != nil {
		fmt.Println("RemoveSingleDrugInfoDB: tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	_, err = stmt.Exec(drug)
	if err != nil {
		fmt.Println("RemoveSingleDrugInfoDB: stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("RemoveSingleDrugInfoDB: tx.Commit():", err)
		return false
	}

	fmt.Println("Data removed from info DB successfully.")

	return true
}

func getTableNamesQuery(driver string, path string) string {
	var queryStr string
	if driver == "sqlite3" {
		queryStr = "SELECT name FROM sqlite_schema WHERE type='table'"
	} else if driver == "mysql" {
		dbName := strings.Split(path, "/")
		queryStr = "SELECT table_name FROM information_schema.tables WHERE table_schema = '" + dbName[1] + "'"
	}
	return queryStr
}

func CheckDBTables(driver string, path string) bool {
	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	queryStr := getTableNamesQuery(driver, path)
	rows, err := db.Query(queryStr)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer rows.Close()

	var tableList []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			fmt.Println(err)
			return false
		}
		tableList = append(tableList, name)
	}

	return len(tableList) != 0
}

func CleanDB(driver string, path string) bool {
	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	queryStr := getTableNamesQuery(driver, path)
	rows, err := db.Query(queryStr)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	fmt.Print("Removing tables: ")
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			fmt.Println(err)
			return false
		}

		fmt.Print(name + ", ")

		_, err = tx.Exec("drop table " + name)
		if err != nil {
			fmt.Println("CleanDB: tx.Exec():", err)
			return false
		}
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("CleanDB: tx.Commit():", err)
		return false
	}

	fmt.Println("\nAll tables removed from DB.")

	return true
}

func AddToInfoDB(source string, subs []DrugInfo, driver string, path string) bool {
	if source == "default" {
		source = default_source
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare("insert into " + source +
		" (drugName, drugRoute, " +
		"threshold, " +
		"lowDoseMin, lowDoseMax, " +
		"mediumDoseMin, mediumDoseMax, " +
		"highDoseMin, highDoseMax, " +
		"doseUnits, " +
		"onsetMin, onsetMax, onsetUnits, " +
		"comeUpMin, comeUpMax, comeUpUnits, " +
		"peakMin, peakMax, peakUnits, " +
		"offsetMin, offsetMax, offsetUnits, " +
		"totalDurMin, totalDurMax, totalDurUnits, " +
		"timeOfFetch) " +
		"values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		fmt.Println("AddToInfoDB: tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	for i := 0; i < len(subs); i++ {
		subs[i].DoseUnits = MatchUnits(subs[i].DoseUnits)
		_, err = stmt.Exec(
			subs[i].DrugName,
			subs[i].DrugRoute,
			subs[i].Threshold,
			subs[i].LowDoseMin,
			subs[i].LowDoseMax,
			subs[i].MediumDoseMin,
			subs[i].MediumDoseMax,
			subs[i].HighDoseMin,
			subs[i].HighDoseMax,
			subs[i].DoseUnits,
			subs[i].OnsetMin,
			subs[i].OnsetMax,
			subs[i].OnsetUnits,
			subs[i].ComeUpMin,
			subs[i].ComeUpMax,
			subs[i].ComeUpUnits,
			subs[i].PeakMin,
			subs[i].PeakMax,
			subs[i].PeakUnits,
			subs[i].OffsetMin,
			subs[i].OffsetMax,
			subs[i].OffsetUnits,
			subs[i].TotalDurMin,
			subs[i].TotalDurMax,
			subs[i].TotalDurUnits,
			time.Now().Unix())
		if err != nil {
			fmt.Println("AddToInfoDB: stmt.Exec():", err)
			return false
		}
	}
	err = tx.Commit()
	if err != nil {
		fmt.Println("AddToInfoDB: tx.Commit():", err)
		return false
	}

	return true
}

// Returns true if successful and false otherwise
// Creates the database
// source - the name of the db, a.k.a. the source of the drug information
func InitDrugDB(source string, driver string, path string) bool {
	if source == "default" {
		source = default_source
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	caseInsensitive := ""
	if driver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	initDBsql := "create table " + source + " (drugName varchar(255)" + caseInsensitive + "not null," +
		"drugRoute varchar(255)" + caseInsensitive + "not null," +
		"threshold real," +
		"lowDoseMin real," +
		"lowDoseMax real," +
		"mediumDoseMin real," +
		"mediumDoseMax real," +
		"highDoseMin real," +
		"highDoseMax real," +
		"doseUnits text" + caseInsensitive + "," +
		"onsetMin real," +
		"onsetMax real," +
		"onsetUnits text" + caseInsensitive + "," +
		"comeUpMin real," +
		"comeUpMax real," +
		"comeUpUnits text" + caseInsensitive + "," +
		"peakMin real," +
		"peakMax real," +
		"peakUnits text" + caseInsensitive + "," +
		"offsetMin real," +
		"offsetMax real," +
		"offsetUnits text" + caseInsensitive + "," +
		"totalDurMin real," +
		"totalDurMax real," +
		"totalDurUnits text" + caseInsensitive + "," +
		"timeOfFetch bigint not null," +
		"primary key (drugName, drugRoute));"

	_, err = db.Exec(initDBsql)
	if err != nil {
		fmt.Println(initDBsql+":", err)
		return false
	}

	fmt.Println("Created: '" + source + "' table for drug info in database.")

	initDBsql = "create table userLogs (timeOfDoseStart bigint not null," +
		"username varchar(255) not null," +
		"timeOfDoseEnd bigint not null," +
		"drugName text " + caseInsensitive + " not null," +
		"dose real not null," +
		"doseUnits text " + caseInsensitive + " not null," +
		"drugRoute text " + caseInsensitive + " not null," +
		"primary key (timeOfDoseStart, username));"

	_, err = db.Exec(initDBsql)
	if err != nil {
		fmt.Println(initDBsql+":", err)
		return false
	}

	fmt.Println("Created: 'userLogs' table in database.")

	return true
}

func MatchDrugName(drugname string) string {
	matches := map[string]string{
		"weed": "Cannabis",
	}

	if len(matches[drugname]) == 0 {
		return drugname
	}

	return matches[drugname]
}

func MatchDrugRoute(drugroute string) string {
	matches := map[string]string{
		"drink":    "oral",
		"drinking": "oral",
	}

	if len(matches[drugroute]) == 0 {
		return drugroute
	}

	return matches[drugroute]
}

func MatchUnits(units string) string {
	matches := map[string]string{
		"µg": "ug",
	}

	if len(matches[units]) == 0 {
		return units
	}

	return matches[units]
}

func (cfg *Config) AddToDoseDB(user string, drug string, route string,
	dose float32, units string, perc float32,
	driver string, path string, source string) bool {

	if source == "default" {
		source = default_source
	}

	if user == "default" {
		user = default_username
	}

	drug = MatchDrugName(drug)
	route = MatchDrugRoute(route)

	if perc != 0 {
		var new_units string

		if strings.ToLower(drug) == "alcohol" && units == "ml" {
			new_units = "mL EtOH"
		}

		if strings.ToLower(drug) == "cannabis" && units == "mg" {
			new_units = "mg (THC)"
		}

		av := alconvert.NewAV()
		av.UserSet.Milliliters = float32(dose)
		av.UserSet.Percent = float32(perc)
		av.CalcGotUnits()
		dose = av.GotUnits() * 10

		if len(new_units) == 0 {
			new_units = units
		}

		fmt.Println("Calculated for",
			drug, ":",
			perc, "%",
			"of",
			av.UserSet.Milliliters, units,
			"to be:", dose, new_units)

		units = new_units
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB("drugName", source, driver, path, xtrs[:], drug, route, units)
	if !ret {
		fmt.Println("Combo of Drug, Route and Units doesn't exist in local DB:",
			drug+", "+route+", "+units)
		return false
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("select count(*) from userLogs where username = ?", user).Scan(&count)
	if err != nil {
		fmt.Println("Error when counting user logs for user:", user)
		fmt.Println(err)
		return false
	}

	if int16(count) >= cfg.MaxLogsPerUser {
		diff := count - int(cfg.MaxLogsPerUser)
		if cfg.AutoRemove {
			RemoveLogs(driver, path, user, diff+1, true, 0)
		} else {
			fmt.Println("User:", user, "has reached the maximum entries per user:", cfg.MaxLogsPerUser,
				"; Not logging.")
			return false
		}
	}

	// Add to log db
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare("insert into userLogs" +
		" (timeOfDoseStart, username, timeOfDoseEnd, drugName, dose, doseUnits, drugRoute) " +
		"values(?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(time.Now().Unix(), user, 0, drug, dose, units, route)
	if err != nil {
		fmt.Println(err)
		return false
	}
	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		return false
	}

	fmt.Println("Dose logged successfully!")

	return true
}

func GetLogs(num int, id int64, user string, all bool, driver string, path string, printit bool) []UserLog {
	if user == "default" {
		user = default_username
	}

	numstr := strconv.Itoa(num)

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	var endstmt string
	if all {
		endstmt = ""
	} else {
		endstmt = " limit " + numstr
	}

	orientation := "asc"
	if num > 0 {
		orientation = "desc"
	}

	var rows *sql.Rows
	if id == 0 {
		rows, err = db.Query("select * from userLogs where username = ? order by timeOfDoseStart "+
			orientation+endstmt, user)
	} else {
		rows, err = db.Query("select * from userLogs where username = ? and timeOfDoseStart = ?", user, id)
	}
	if err != nil {
		fmt.Println("GetLogs() Query:", err)
		return nil
	}
	defer rows.Close()

	userlogs := []UserLog{}
	for rows.Next() {
		tempul := UserLog{}
		err = rows.Scan(&tempul.StartTime, &tempul.Username, &tempul.EndTime, &tempul.DrugName,
			&tempul.Dose, &tempul.DoseUnits, &tempul.DrugRoute)
		if err != nil {
			fmt.Println("GetLogs() Scan:", err)
			return nil
		}

		location, err := time.LoadLocation("Local")
		if err != nil {
			fmt.Println("GetLogs() LoadLocation:", err)
			return nil
		}

		if printit {
			fmt.Printf("Start:\t%s (%d) < ID\n",
				time.Unix(int64(tempul.StartTime), 0).In(location), tempul.StartTime)
			if tempul.EndTime != 0 {
				fmt.Printf("End:\t%s (%d)\n",
					time.Unix(int64(tempul.EndTime), 0).In(location), tempul.EndTime)
			}
			fmt.Printf("Drug:\t%s\n", tempul.DrugName)
			fmt.Printf("Dose:\t%f %s\n", tempul.Dose, tempul.DoseUnits)
			fmt.Printf("Route:\t%s\n", tempul.DrugRoute)
			fmt.Printf("User:\t%s\n", tempul.Username)
			fmt.Println("=========================")
		}

		userlogs = append(userlogs, tempul)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println("GetLogs() rows.Err():", err)
		return nil
	}
	if len(userlogs) == 0 {
		return nil
	}
	return userlogs
}

func GetLocalInfo(drug string, source string, driver string, path string, printit bool) []DrugInfo {
	if source == "default" {
		source = default_source
	}

	drug = MatchDrugName(drug)

	ret := checkIfExistsDB("drugName", source, driver, path, nil, drug)
	if !ret {
		fmt.Println("No such drug in info database:", drug)
		return nil
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	rows, err := db.Query("select * from "+source+" where drugName = ?", drug)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer rows.Close()
	infoDrug := []DrugInfo{}
	for rows.Next() {
		tempdrinfo := DrugInfo{}
		err := rows.Scan(&tempdrinfo.DrugName, &tempdrinfo.DrugRoute,
			&tempdrinfo.Threshold,
			&tempdrinfo.LowDoseMin, &tempdrinfo.LowDoseMax, &tempdrinfo.MediumDoseMin,
			&tempdrinfo.MediumDoseMax, &tempdrinfo.HighDoseMin, &tempdrinfo.HighDoseMax,
			&tempdrinfo.DoseUnits, &tempdrinfo.OnsetMin, &tempdrinfo.OnsetMax,
			&tempdrinfo.OnsetUnits, &tempdrinfo.ComeUpMin, &tempdrinfo.ComeUpMax,
			&tempdrinfo.ComeUpUnits, &tempdrinfo.PeakMin, &tempdrinfo.PeakMax,
			&tempdrinfo.PeakUnits, &tempdrinfo.OffsetMin, &tempdrinfo.OffsetMax,
			&tempdrinfo.OffsetUnits, &tempdrinfo.TotalDurMin, &tempdrinfo.TotalDurMax,
			&tempdrinfo.TotalDurUnits, &tempdrinfo.TimeOfFetch)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		location, err := time.LoadLocation("Local")
		if err != nil {
			fmt.Println(err)
		}

		if printit {
			fmt.Println("Drug:", tempdrinfo.DrugName, ";", "Route:", tempdrinfo.DrugRoute)
			fmt.Println("---Dosages---")
			fmt.Printf("Threshold: %g\n", tempdrinfo.Threshold)
			fmt.Println("Min\tMax\tRange")
			fmt.Printf("%g\t%g\tLow\n", tempdrinfo.LowDoseMin, tempdrinfo.LowDoseMax)
			fmt.Printf("%g\t%g\tMedium\n", tempdrinfo.MediumDoseMin, tempdrinfo.MediumDoseMax)
			fmt.Printf("%g\t%g\tHigh\n", tempdrinfo.HighDoseMin, tempdrinfo.HighDoseMax)
			fmt.Println("Dose units:", tempdrinfo.DoseUnits)
			fmt.Println("---Times---")
			fmt.Println("Min\tMax\tPeriod\tUnits")
			fmt.Printf("%g\t%g\tOnset\t%s\n",
				tempdrinfo.OnsetMin,
				tempdrinfo.OnsetMax,
				tempdrinfo.OnsetUnits)
			fmt.Printf("%g\t%g\tComeup\t%s\n",
				tempdrinfo.ComeUpMin,
				tempdrinfo.ComeUpMax,
				tempdrinfo.ComeUpUnits)
			fmt.Printf("%g\t%g\tPeak\t%s\n",
				tempdrinfo.PeakMin,
				tempdrinfo.PeakMax,
				tempdrinfo.PeakUnits)
			fmt.Printf("%g\t%g\tOffset\t%s\n",
				tempdrinfo.OffsetMin,
				tempdrinfo.OffsetMax,
				tempdrinfo.OffsetUnits)
			fmt.Printf("%g\t%g\tTotal\t%s\n",
				tempdrinfo.TotalDurMin,
				tempdrinfo.TotalDurMax,
				tempdrinfo.TotalDurUnits)
			fmt.Println("Time of fetch:", time.Unix(int64(tempdrinfo.TimeOfFetch), 0).In(location))
			fmt.Println("====================")
		}

		infoDrug = append(infoDrug, tempdrinfo)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return infoDrug
}

func RemoveLogs(driver string, path string, username string, amount int, reverse bool, remID int) bool {
	if username == "default" {
		username = default_username
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	stmtStr := "delete from userLogs where username = ?"
	if amount != 0 {
		direction := "desc"
		if reverse {
			direction = "asc"
		}

		selectStr := fmt.Sprintf("select timeOfDoseStart from userLogs where username = ? "+
			"order by timeOfDoseStart %s limit %d", direction, amount)

		rows, err := db.Query(selectStr, username)
		if err != nil {
			fmt.Println("Error Select for RemoveLogs():", err)
			return false
		}

		var gotTimeOfDose []int
		var tempTimes int

		for rows.Next() {
			err = rows.Scan(&tempTimes)
			if err != nil {
				fmt.Println("Error scanning Select for RemoveLogs():", err)
				return false
			}
			gotTimeOfDose = append(gotTimeOfDose, tempTimes)
		}

		concatTimes := ""
		for i := 0; i < len(gotTimeOfDose); i++ {
			concatTimes = concatTimes + strconv.Itoa(gotTimeOfDose[i]) + ","
		}
		concatTimes = strings.TrimSuffix(concatTimes, ",")

		stmtStr = "delete from userLogs where timeOfDoseStart in (" + concatTimes + ")"
	} else if remID != 0 {
		stmtStr = "delete from userLogs where timeOfDoseStart = ? AND username = ?"
	}

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare(stmtStr)
	if err != nil {
		fmt.Println("RemoveLogs: tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	if remID != 0 {
		_, err = stmt.Exec(remID, username)
	} else if amount != 0 {
		_, err = stmt.Exec()
	} else {
		_, err = stmt.Exec(username)
	}
	if err != nil {
		fmt.Println("RemoveLogs: stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("RemoveLogs: tx.Commit():", err)
		return false
	}

	fmt.Println("Data removed from info DB successfully.")

	return true
}

func SetEndTime(driver string, path string, username string, id int, customTime int) bool {
	if username == "default" {
		username = default_username
	}

	db, err := sql.Open(driver, path)
	if err != nil {
		errorCantOpenDB(path, err)
	}
	defer db.Close()

	endStmt := "order by timeOfDoseStart desc"
	if id != 0 {
		endStmt = "and timeOfDoseStart = ?"
	}

	selectStr := fmt.Sprintf("select timeOfDoseStart from userLogs where username = ? "+"%s limit 1",
		endStmt)

	var gotTimeOfDose int

	if id != 0 {
		err = db.QueryRow(selectStr, username, id).Scan(&gotTimeOfDose)
	} else {
		err = db.QueryRow(selectStr, username).Scan(&gotTimeOfDose)
	}

	if err != nil {
		fmt.Println("Error Select for SetEndTime:", err)
		return false
	}

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	var theTime int
	if customTime != 0 {
		theTime = customTime
	} else {
		theTime = int(time.Now().Unix())
	}

	stmtStr := fmt.Sprintf("update userLogs set timeOfDoseEnd = %d where timeOfDoseStart = ?",
		theTime)

	stmt, err := tx.Prepare(stmtStr)
	if err != nil {
		fmt.Println("SetEndTime: tx.Prepare():", err)
		return false
	}
	defer stmt.Close()
	_, err = stmt.Exec(gotTimeOfDose)

	if err != nil {
		fmt.Println("SetEndTime: stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("SetEndTime: tx.Commit():", err)
		return false
	}

	fmt.Println("End time set for entry.")

	return true
}
