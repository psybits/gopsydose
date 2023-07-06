package drugdose

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
)

const test_drug string = "test_drug"
const test_route string = "test_route"
const test_units string = "test_units"
const test_user string = "test_user"
const test_source string = "test"

func testWithDrivers() [2]string {
	return [2]string{"sqlite3", "mysql"}
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

	db := gotsetcfg.OpenDBConnection(ctx)

	gotsetcfg.InitAllDB(db, ctx)

	var testsub []DrugInfo
	tempsub := DrugInfo{
		DrugName:  test_drug,
		DrugRoute: test_route,
		DoseUnits: test_units,
	}
	testsub = append(testsub, tempsub)

	ret := gotsetcfg.AddToInfoDB(db, ctx, testsub)
	if ret == false {
		os.Exit(1)
	}

	return db, ctx, gotsetcfg
}

func (cfg Config) cleanAfterTest(db *sql.DB, ctx context.Context) {
	ret := cfg.CleanInfo(db, ctx)
	if ret == false {
		os.Exit(1)
	}

	ret = cfg.CleanNames(db, ctx, true)
	if ret == false {
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
	for _, v := range testWithDrivers() {
		db, ctx, cfg := initForTests(v)
		defer db.Close()

		temp_doses := genLogDoses()
		temp_users := genLogUsers()

		// Test valid logs
		for o := 0; o < 2; o++ {
			fmt.Println("\t=== Testing", testUsernames(o), "usernames ===")

			errorChannel := make(chan error)
			count := 0
			for count < 5 {
				go cfg.AddToDoseDB(db, ctx, errorChannel, temp_users[useUser(count, o)], test_drug,
					test_route, temp_doses[count], test_units, 0, true)
				gotErr := <-errorChannel
				if gotErr != nil {
					t.Log(gotErr)
					t.Fail()
					break
				}
				count++
			}

			logsChannel := make(chan []UserLog)
			for i := 0; i < count; i++ {
				go cfg.GetLogs(db, logsChannel, errorChannel, ctx, count,
					0, temp_users[useUser(i, o)], true, "")
			}

			for i := 0; i < count; i++ {
				gotLog := <-logsChannel
				gotErr := <-errorChannel
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
				go cfg.RemoveLogs(db, ctx, errorChannel, temp_users[useUser(i, o)], 1, true, 0, "")
				gotErr := <-errorChannel
				if gotErr != nil {
					fmt.Println(gotErr)
				}
			}
		}

		// Test invalid logs
		logsChannel := make(chan []UserLog)
		errorChannel := make(chan error)
		for i := 0; i < 5; i++ {
			go cfg.GetLogs(db, logsChannel, errorChannel, ctx, 1,
				0, "W2IK&m9)abN8*(x9Ms90mMm", true, "")
		}

		for i := 0; i < 5; i++ {
			gotLog := <-logsChannel
			gotErr := <-errorChannel
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
