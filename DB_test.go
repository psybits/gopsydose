package drugdose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

const test_drug string = "test_drug"
const test_route string = "test_route"
const test_units string = "test_units"
const test_user string = "test_user"
const test_source string = "test"

func testWithDrivers() [2]string {
	return [2]string{SqliteDriver, MysqlDriver}
}

func testUsernames(o int) string {
	if o == 0 {
		return "same"
	} else if o == 1 {
		return "different"
	}
	return ""
}

func useUser(i int, o int) int {
	if testUsernames(o) == "same" {
		return 0
	}
	return i
}

func initForTests(dbDriver string) (*sql.DB, context.Context, Config) {
	gotsetcfg := InitAllSettings("test", DefaultDBDir, DefaultDBName,
		DefaultMySQLAccess, false, false, true, "test")

	gotsetcfg.AutoFetch = false
	gotsetcfg.AutoRemove = false
	gotsetcfg.UseSource = test_source
	gotsetcfg.DBDriver = dbDriver

	fmt.Println("\tinitForTests: DBDriver:", gotsetcfg.DBDriver)

	ctx := context.Background()

	gotsetcfg.InitAllDB(ctx)

	db := gotsetcfg.OpenDBConnection(ctx)

	err := gotsetcfg.AddToAllNamesTables(db, ctx, false)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var testsub []DrugInfo
	tempsub := DrugInfo{
		DrugName:  test_drug,
		DrugRoute: test_route,
		DoseUnits: test_units,
	}
	testsub = append(testsub, tempsub)

	errChannel := make(chan ErrorInfo)
	go gotsetcfg.AddToInfoTable(db, ctx, errChannel, testsub, "")
	errInfo := <-errChannel
	err = errInfo.Err
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return db, ctx, gotsetcfg
}

func (cfg Config) cleanAfterTest(db *sql.DB, ctx context.Context) {
	err := cfg.CleanInfoTable(db, ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = cfg.CleanNamesTables(db, ctx, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func logIsInvalid(ulogs []UserLog, uerr error, temp_doses []float32, temp_users []string, count int) bool {
	if uerr != nil || ulogs == nil {
		return true
	}

	found_count := 0
	for i := 0; i < count; i++ {
		for u := 0; u < len(temp_doses); u++ {
			for y := 0; y < len(temp_users); y++ {
				if ulogs[i].DrugName == test_drug &&
					ulogs[i].DrugRoute == test_route &&
					ulogs[i].DoseUnits == test_units &&
					ulogs[i].Dose == temp_doses[u] &&
					ulogs[i].Username == temp_users[y] {
					found_count++
					break
				}
			}
		}
	}

	return found_count != count
}

func genLogDoses() []float32 {
	return []float32{1.12, 2.12, 3.12, 4.12, 5.12}
}

func genLogUsers() []string {
	var temp_users []string
	for i := 0; i < 5; i++ {
		temp_users = append(temp_users, test_user+"_"+strconv.Itoa(i))
	}
	return temp_users
}

func TestConcurrentGetLogs(t *testing.T) {
	fmt.Println("\t---Starting TestConcurrentGetLogs()")
	for _, v := range testWithDrivers() {
		db, ctx, cfg := initForTests(v)
		defer db.Close()

		temp_doses := genLogDoses()
		temp_users := genLogUsers()

		// Test valid logs
		for o := 0; o < 2; o++ {
			fmt.Println("\t=== Testing", testUsernames(o), "usernames ===")

			synct := SyncTimestamps{}
			errorChannel := make(chan ErrorInfo)
			count := 0
			for count < 5 {
				go cfg.AddToDoseTable(db, ctx, errorChannel, &synct, temp_users[useUser(count, o)], test_drug,
					test_route, temp_doses[count], test_units, 0, 0, "", true)
				gotErrInfo := <-errorChannel
				if gotErrInfo.Err != nil {
					fmt.Println("\tFailed adding to database.")
					t.Log(gotErrInfo.Err)
					t.Fail()
					break
				}
				count++
			}

			userLogsErrChan := make(chan UserLogsError)
			for i := 0; i < count; i++ {
				go cfg.GetLogs(db, ctx, userLogsErrChan, count,
					0, temp_users[useUser(i, o)], true, "", "")
			}

			for i := 0; i < count; i++ {
				gotUserLogsErr := <-userLogsErrChan
				gotLog := gotUserLogsErr.UserLogs
				gotErr := gotUserLogsErr.Err
				snd_count := count
				if testUsernames(o) == "different" {
					snd_count = 1
				}
				if logIsInvalid(gotLog, gotErr, temp_doses, temp_users, snd_count) {
					fmt.Println("\tFailed reading database.")
					t.Log("Didn't read database properly concurrently, breaking. ; err:", gotErr)
					t.Fail()
					break
				}
			}

			for i := 0; i < count; i++ {
				go cfg.RemoveLogs(db, ctx, errorChannel, temp_users[useUser(i, o)], 1, true, 0, "", "")
				gotErrInfo := <-errorChannel
				if gotErrInfo.Err != nil {
					fmt.Println(gotErrInfo.Err)
				}
			}
		}

		// Test invalid logs
		userLogsErrChan := make(chan UserLogsError)
		for i := 0; i < 5; i++ {
			go cfg.GetLogs(db, ctx, userLogsErrChan, 1,
				0, "W2IK&m9)abN8*(x9Ms90mMm", true, "", "")
		}

		for i := 0; i < 5; i++ {
			gotUserLogsErr := <-userLogsErrChan
			gotLog := gotUserLogsErr.UserLogs
			gotErr := gotUserLogsErr.Err
			if gotErr != nil {
				fmt.Println("Testing invalid username:", gotErr, "; gotLog:", gotLog)
			} else if gotErr == nil {
				t.Log("getErr is nil, when it should've returned an error")
				t.Fail()
			}
		}

		cfg.cleanAfterTest(db, ctx)
	}
}

func TestConcurrentAddToDoseTable(t *testing.T) {
	fmt.Println("\t---Starting TestConcurrentAddToDoseTable()")
	for _, v := range testWithDrivers() {
		db, ctx, cfg := initForTests(v)
		defer db.Close()

		temp_doses := genLogDoses()
		temp_users := genLogUsers()

		// Test valid logs
		for o := 0; o < 2; o++ {
			fmt.Println("\t=== Testing", testUsernames(o), "usernames ===")

			synct := SyncTimestamps{}
			errorChannel := make(chan ErrorInfo)
			for i := 0; i < 5; i++ {
				go cfg.AddToDoseTable(db, ctx, errorChannel, &synct, temp_users[useUser(i, o)], test_drug,
					test_route, temp_doses[i], test_units, 0, 0, "", true)
			}

			count := 0
			for count < 5 {
				gotErrInfo := <-errorChannel
				if gotErrInfo.Err != nil {
					fmt.Println("\tFailed adding to database.")
					t.Log(gotErrInfo.Err)
					t.Fail()
					break
				}
				count++
			}

			userLogsErrChan := make(chan UserLogsError)
			for i := 0; i < count; i++ {
				go cfg.GetLogs(db, ctx, userLogsErrChan, count,
					0, temp_users[useUser(i, o)], true, "", "")
				gotUserLogsErr := <-userLogsErrChan
				gotLog := gotUserLogsErr.UserLogs
				gotErr := gotUserLogsErr.Err
				snd_count := count
				if testUsernames(o) == "different" {
					snd_count = 1
				}
				if logIsInvalid(gotLog, gotErr, temp_doses, temp_users, snd_count) {
					fmt.Println("\tFailed reading database.")
					t.Log("Didn't read database properly concurrently, breaking. ; err:", gotErr)
					t.Fail()
					break
				}
			}

			for i := 0; i < count; i++ {
				go cfg.RemoveLogs(db, ctx, errorChannel, temp_users[useUser(i, o)], 1, true, 0, "", "")
				gotErrInfo := <-errorChannel
				if gotErrInfo.Err != nil {
					fmt.Println(gotErrInfo.Err)
				}
			}
		}

		// Test invalid logs
		synct := SyncTimestamps{}
		errorChannel := make(chan ErrorInfo)
		for i := 0; i < 5; i++ {
			go cfg.AddToDoseTable(db, ctx, errorChannel, &synct, "test_user", "W2IK&m9)abN\"8*(x9Ms90mMm",
				"W2IK&m9)abN\"8*(x9Ms90mMm", 123.12, "W2IK&m9)abN\"8*(x9Ms90mMm", 0, 0, "", true)
		}

		for i := 0; i < 5; i++ {
			gotErrInfo := <-errorChannel
			if gotErrInfo.Err != nil {
				fmt.Println("Testing invalid input:", gotErrInfo.Err)
			} else if gotErrInfo.Err == nil {
				t.Log("getErrInfo is nil, when it should've returned an error")
				t.Fail()
			}
		}

		cfg.cleanAfterTest(db, ctx)
	}
}

func TestUseConfigTimeout(t *testing.T) {
	fmt.Println("\t---Starting TestUseConfigTimeout()")
	db, ctx, cfg := initForTests(SqliteDriver)
	defer db.Close()

	cfg.Timeout = "1s"

	ctx2, cancel, err := cfg.UseConfigTimeout()
	defer cancel()
	if err != nil {
		cfg.cleanAfterTest(db, ctx)
		t.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	userLogsErrChan := make(chan UserLogsError)
	go cfg.GetLogs(db, ctx2, userLogsErrChan, 1, 0, test_user,
		false, "", "")
	gotUserLogsErr := <-userLogsErrChan
	gotErr := gotUserLogsErr.Err

	if gotErr != nil && errors.Is(gotErr, context.DeadlineExceeded) == false {
		t.Log("Got wrong error message:", gotErr)
		t.Fail()
	}

	if gotErr == nil {
		t.Log("There should've been an error, but there is none:", gotErr)
		t.Fail()
	}

	fmt.Println(gotErr)

	cfg.cleanAfterTest(db, ctx)
}

func TestForcedRollback(t *testing.T) {
	fmt.Println("\t---Starting TestForcedRollback()")
	for _, v := range testWithDrivers() {
		db, ctx, cfg := initForTests(v)
		defer db.Close()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			cfg.cleanAfterTest(db, ctx)
			t.Fatal(err)
		}

		err, set := settingsTables(settingTypeID)
		if err != nil {
			cfg.cleanAfterTest(db, ctx)
			t.Fatal(err)
		}

		stmtStr := returnSetUserSetStmt(set)

		err = errors.New("Test error.")
		errChannel := make(chan ErrorInfo)
		printN := "testNamed"

		tempErrInfo := ErrorInfo{
			Err:      nil,
			Action:   "testing",
			Username: "testing",
		}

		fmt.Println("\tConcurrent rollback")
		stmt, _ := tx.Prepare(stmtStr)
		defer stmt.Close()
		var testConcurrentRollback = func(err error, tx *sql.Tx,
			errChannel chan ErrorInfo, errInfo ErrorInfo, printN string,
			xtra string, t *testing.T) {
			if handleErrRollback(err, tx, errChannel, errInfo, printN, xtra) {
				fmt.Println("handleErrRollback():", err)
			} else {
				t.Log("handleErrRollback():", err)
				t.Fail()
			}
		}
		go testConcurrentRollback(err, tx, errChannel, tempErrInfo,
			printN, "tx.Prepare(): ", t)
		gotErrInfo := <-errChannel
		fmt.Println("errChannel:", gotErrInfo.Err)
		err = tx.Commit()
		if err != nil {
			fmt.Println("tx.Commit():", err)
		} else {
			t.Log("forced rollback err shouldn't have been nil: tx.Commit():", err)
			t.Fail()
		}
		// End of concurrent rollback

		fmt.Println("\tSequential rollback")
		tx, err = db.BeginTx(ctx, nil)
		if err != nil {
			cfg.cleanAfterTest(db, ctx)
			t.Fatal(err)
		}
		err = errors.New("Test error.")
		stmt, _ = tx.Prepare(stmtStr)
		err = handleErrRollbackSeq(err, tx, printN, "tx.Prepare(): ")
		if err != nil {
			fmt.Println("handleErrRollbackSeq():", err)
		} else {
			t.Log("handleErrRollbackSeq():", err)
			t.Fail()
		}
		defer stmt.Close()
		err = tx.Commit()
		if err != nil {
			fmt.Println("tx.Commit():", err)
		} else {
			t.Log("tx.Commit():", err)
			t.Fail()
		}
		// End of sequential rollback

		cfg.cleanAfterTest(db, ctx)
	}
}
