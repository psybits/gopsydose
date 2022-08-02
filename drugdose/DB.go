package drugdose

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"database/sql"

	// MySQL driver needed for sql module
	_ "github.com/go-sql-driver/mysql"

	// SQLite driver needed for sql module
	_ "github.com/mattn/go-sqlite3"

	"github.com/powerjungle/goalconvert/alconvert"
)

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

// InitDBFileStructure creates the basic file structure for the database.
// This should be run only once!
func (cfg Config) InitDBFileStructure() bool {
	ret := cfg.checkDBFileStruct()
	if ret == true {
		return true
	}

	dirOnly := path.Dir(cfg.DBSettings[cfg.DBDriver].Path)

	err := os.Mkdir(dirOnly, 0700)
	if err != nil {
		fmt.Println("Error creating directory for DB:", err)
		exitProgram()
	}

	dbFileLocat := cfg.DBSettings[cfg.DBDriver].Path

	file, err := os.Create(dbFileLocat)
	if err != nil {
		errorCantCreateDB(dbFileLocat, err)
	}

	err = file.Close()
	if err != nil {
		errorCantCloseDB(dbFileLocat, err)
	}

	fmt.Println("Initialised the DB file structure.")

	return true
}

// checkDBFileStruct Returns true if the file structure is already created,
// false otherwise. Checks whether the db directory and minimum amount of files
// exist with the proper names in it.
func (cfg Config) checkDBFileStruct() bool {
	dbFileLocat := cfg.DBSettings[cfg.DBDriver].Path

	if _, err := os.Stat(dbFileLocat); err == nil {
		VerbosePrint(dbFileLocat+": Exists", cfg.VerbosePrinting)
	} else if errors.Is(err, os.ErrNotExist) {
		fmt.Println(dbFileLocat+": Doesn't seem to exist:", err)
		return false
	}

	return true
}

// RemoveSingleDrugInfoDB Remove all entries of a single drug from the local info DB, instead of deleting the whole DB.
func (cfg Config) RemoveSingleDrugInfoDB(drug string) bool {
	drug = MatchDrugName(drug)

	ret := checkIfExistsDB("drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		fmt.Println("No such drug in info database:", drug)
		return false
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare("delete from " + cfg.UseSource +
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

func (cfg Config) getTableNamesQuery() string {
	var queryStr string
	if cfg.DBDriver == "sqlite3" {
		queryStr = "SELECT name FROM sqlite_schema WHERE type='table'"
	} else if cfg.DBDriver == "mysql" {
		dbName := strings.Split(cfg.DBSettings[cfg.DBDriver].Path, "/")
		queryStr = "SELECT table_name FROM information_schema.tables WHERE table_schema = '" + dbName[1] + "'"
	}
	return queryStr
}

func (cfg Config) CheckDBTables() bool {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	queryStr := cfg.getTableNamesQuery()
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

func (cfg Config) CleanDB() bool {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	queryStr := cfg.getTableNamesQuery()
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

func (cfg Config) AddToInfoDB(subs []DrugInfo) bool {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return false
	}

	stmt, err := tx.Prepare("insert into " + cfg.UseSource +
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

// InitDrugDB Returns true if successful and false otherwise
// Creates the database
// source - the name of the db, a.k.a. the source of the drug information
func (cfg Config) InitDrugDB() bool {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	caseInsensitive := ""
	if cfg.DBDriver == "sqlite3" {
		caseInsensitive = " COLLATE NOCASE "
	}

	initDBsql := "create table " + cfg.UseSource + " (drugName varchar(255)" + caseInsensitive + "not null," +
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

	fmt.Println("Created: '" + cfg.UseSource + "' table for drug info in database.")

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

func (cfg Config) AddToDoseDB(user string, drug string, route string,
	dose float32, units string, perc float32) bool {

	drug = MatchDrugName(drug)
	route = MatchDrugRoute(route)

	if perc != 0 {
		var newUnits string

		if strings.ToLower(drug) == "alcohol" && units == "ml" {
			newUnits = "mL EtOH"
		}

		if strings.ToLower(drug) == "cannabis" && units == "mg" {
			newUnits = "mg (THC)"
		}

		av := alconvert.NewAV()
		av.UserSet.Milliliters = float32(dose)
		av.UserSet.Percent = float32(perc)
		av.CalcGotUnits()
		dose = av.GotUnits() * 10

		if len(newUnits) == 0 {
			newUnits = units
		}

		fmt.Println("Calculated for",
			drug, ":",
			perc, "%",
			"of",
			av.UserSet.Milliliters, units,
			"to be:", dose, newUnits)

		units = newUnits
	}

	xtrs := [2]string{xtrastmt("drugRoute", "and"), xtrastmt("doseUnits", "and")}
	ret := checkIfExistsDB("drugName", cfg.UseSource,
		cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
		xtrs[:], drug, route, units)
	if !ret {
		fmt.Println("Combo of Drug, Route and Units doesn't exist in local DB:",
			drug+", "+route+", "+units)
		return false
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
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
			cfg.RemoveLogs(user, diff+1, true, 0, "none")
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

func (cfg Config) GetDBSize() int64 {
	if cfg.DBDriver == "sqlite3" {
		file, err := os.Open(cfg.DBSettings[cfg.DBDriver].Path)
		if err != nil {
			fmt.Println("GetDBSize: error opening:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		fileInfo, err := file.Stat()
		if err != nil {
			fmt.Println("GetDBSize: error getting stat:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		err = file.Close()
		if err != nil {
			fmt.Println("GetDBSize: error closing file:", cfg.DBSettings[cfg.DBDriver].Path, ":", err)
			return 0
		}

		return fileInfo.Size()
	} else if cfg.DBDriver == "mysql" {
		db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
		if err != nil {
			errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
		}
		defer db.Close()

		res := strings.Split(cfg.DBSettings[cfg.DBDriver].Path, "/")
		dbName := res[1]

		dbSizeQuery := "select SUM(data_length + index_length) FROM information_schema.tables " +
			"where table_schema = ?"

		var totalSize int64

		row := db.QueryRow(dbSizeQuery, dbName)
		err = row.Scan(&totalSize)
		if err != nil {
			fmt.Println("GetDBSize: error getting size:", err)
			return 0
		}

		return totalSize
	}

	fmt.Println("GetDBSize: the chosen driver is not a proper one:", cfg.DBDriver)
	return 0
}

func (cfg Config) GetUsers() []string {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var allUsers []string
	var tempUser string

	rows, err := db.Query("select distinct username from userLogs")
	if err != nil {
		fmt.Println("GetUsers: Query: error getting usernames:", err)
		return nil
	}

	for rows.Next() {
		err = rows.Scan(&tempUser)
		if err != nil {
			fmt.Println("GetUsers: Scan: error getting usernames:", err)
			return nil
		}
		allUsers = append(allUsers, tempUser)
	}

	return allUsers
}

func (cfg Config) GetLogsCount(user string) int {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var count int

	row := db.QueryRow("select count(*) from userLogs where username = ?", user)
	err = row.Scan(&count)
	if err != nil {
		fmt.Println("GetLogsCount: error getting count:", err)
		return 0
	}

	return count
}

func (cfg Config) GetLogs(num int, id int64, user string, all bool,
	reverse bool, printit bool, search string) []UserLog {

	numstr := strconv.Itoa(num)

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	var endstmt string
	if all {
		endstmt = ""
	} else {
		endstmt = " limit " + numstr
	}

	orientation := "asc"
	if reverse {
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

		match := true
		if search != "none" {
			var tempArr = [7]string{
				strconv.FormatInt(tempul.StartTime, 10),
				tempul.Username,
				strconv.FormatInt(tempul.EndTime, 10),
				tempul.DrugName,
				strconv.FormatFloat(float64(tempul.Dose), 'f', 2, 32),
				tempul.DoseUnits,
				tempul.DrugRoute,
			}
			match = false
			for i := 0; i < len(tempArr); i++ {
				if strings.Contains(tempArr[i], search) {
					match = true
					break
				}
			}
		}

		if match {
			if printit {
				fmt.Printf("Start:\t%q (%d) < ID\n",
					time.Unix(int64(tempul.StartTime), 0).In(location), tempul.StartTime)
				if tempul.EndTime != 0 {
					fmt.Printf("End:\t%q (%d)\n",
						time.Unix(int64(tempul.EndTime), 0).In(location), tempul.EndTime)
				}
				fmt.Printf("Drug:\t%q\n", tempul.DrugName)
				fmt.Printf("Dose:\t%f %q\n", tempul.Dose, tempul.DoseUnits)
				fmt.Printf("Route:\t%q\n", tempul.DrugRoute)
				fmt.Printf("User:\t%q\n", tempul.Username)
				fmt.Println("=========================")
			}

			userlogs = append(userlogs, tempul)
		}
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

func (cfg Config) GetLocalInfoNames() []string {
	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	rows, err := db.Query("select distinct drugName from " + cfg.UseSource)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer rows.Close()

	var drugList []string
	for rows.Next() {
		var holdName string
		err := rows.Scan(&holdName)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		drugList = append(drugList, holdName)
	}
	err = rows.Err()
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return drugList
}

func (cfg Config) GetLocalInfo(drug string, printit bool) []DrugInfo {
	drug = MatchDrugName(drug)

	ret := checkIfExistsDB("drugName",
		cfg.UseSource,
		cfg.DBDriver,
		cfg.DBSettings[cfg.DBDriver].Path,
		nil,
		drug)
	if !ret {
		fmt.Println("No such drug in info database:", drug)
		return nil
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	rows, err := db.Query("select * from "+cfg.UseSource+" where drugName = ?", drug)
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
			fmt.Printf("%g\t%g\tOnset\t%q\n",
				tempdrinfo.OnsetMin,
				tempdrinfo.OnsetMax,
				tempdrinfo.OnsetUnits)
			fmt.Printf("%g\t%g\tComeup\t%q\n",
				tempdrinfo.ComeUpMin,
				tempdrinfo.ComeUpMax,
				tempdrinfo.ComeUpUnits)
			fmt.Printf("%g\t%g\tPeak\t%q\n",
				tempdrinfo.PeakMin,
				tempdrinfo.PeakMax,
				tempdrinfo.PeakUnits)
			fmt.Printf("%g\t%g\tOffset\t%q\n",
				tempdrinfo.OffsetMin,
				tempdrinfo.OffsetMax,
				tempdrinfo.OffsetUnits)
			fmt.Printf("%g\t%g\tTotal\t%q\n",
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

func (cfg Config) RemoveLogs(username string, amount int, reverse bool,
	remID int64, search string) bool {

	stmtStr := "delete from userLogs where username = ?"
	if amount != 0 && remID == 0 || search != "none" {
		getAll := false
		if search != "none" {
			getAll = true
		}

		gotLogs := cfg.GetLogs(amount, 0, username, getAll, reverse, false, search)
		if gotLogs == nil {
			fmt.Println("RemoveLogs: couldn't get logs, because of an error, no logs will be removed.")
			return false
		}

		var gotTimeOfDose []int64
		var tempTimes int64

		for i := 0; i < len(gotLogs); i++ {
			tempTimes = gotLogs[i].StartTime
			gotTimeOfDose = append(gotTimeOfDose, tempTimes)
		}

		concatTimes := ""
		for i := 0; i < len(gotTimeOfDose); i++ {
			concatTimes = concatTimes + strconv.FormatInt(gotTimeOfDose[i], 10) + ","
		}
		concatTimes = strings.TrimSuffix(concatTimes, ",")

		stmtStr = "delete from userLogs where timeOfDoseStart in (" + concatTimes + ")"
	} else if remID != 0 && search == "none" {
		xtrs := [1]string{xtrastmt("username", "and")}
		ret := checkIfExistsDB("timeOfDoseStart", "userLogs",
			cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path,
			xtrs[:], remID, username)
		if !ret {
			fmt.Println("Log with ID:", remID, "doesn't exists.")
			return false
		}

		stmtStr = "delete from userLogs where timeOfDoseStart = ? AND username = ?"
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

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
	} else if amount != 0 || search != "none" {
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

func (cfg Config) SetUserLogs(set string, id int64, username string, setValue string) bool {
	if username == "none" {
		fmt.Println("Set: Please specify an username!")
		return false
	}

	if set == "none" {
		fmt.Println("Set: Please specify a set type!")
		return false
	}

	if setValue == "none" {
		fmt.Println("Set: Please specify a value to set!")
		return false
	}

	if setValue == "now" && set == "start-time" || setValue == "now" && set == "end-time" {
		setValue = strconv.FormatInt(time.Now().Unix(), 10)
	}

	if set == "start-time" || set == "end-time" {
		if _, err := strconv.ParseInt(setValue, 10, 64); err != nil {
			fmt.Println("Set: error when checking if integer:", err)
			return false
		}
	}

	if set == "dose" {
		if _, err := strconv.ParseFloat(setValue, 64); err != nil {
			fmt.Println("Set: error when checking if float:", err)
			return false
		}
	}

	setName := map[string]string{
		"start-time": "timeOfDoseStart",
		"end-time":   "timeOfDoseEnd",
		"drug":       "drugName",
		"dose":       "dose",
		"units":      "doseUnits",
		"route":      "drugRoute",
	}

	db, err := sql.Open(cfg.DBDriver, cfg.DBSettings[cfg.DBDriver].Path)
	if err != nil {
		errorCantOpenDB(cfg.DBSettings[cfg.DBDriver].Path, err)
	}
	defer db.Close()

	if id == 0 {
		findIdStmt := "select timeOfDoseStart from userLogs where username = ? " +
			"order by timeOfDoseStart desc limit 1"
		err = db.QueryRow(findIdStmt, username).Scan(&id)
		if err != nil {
			fmt.Println("Set: findIdStmt:", err)
			return false
		}
	}

	stmtStr := fmt.Sprintf("update userLogs set %s = ? where timeOfDoseStart = ?",
		setName[set])

	tx, err := db.Begin()
	if err != nil {
		fmt.Println("Set: db.Begin():", err)
		return false
	}

	stmt, err := tx.Prepare(stmtStr)
	if err != nil {
		fmt.Println("Set: tx.Prepare():", err)
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(setValue, id)

	if err != nil {
		fmt.Println("Set: stmt.Exec():", err)
		return false
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println("Set: tx.Commit():", err)
		return false
	}

	fmt.Println(set + ": set for entry.")

	return true
}
