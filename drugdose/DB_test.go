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

func logIsInvalid(ulogs []UserLog, temp_doses []float32, temp_users []string, count int) bool {
	found_count := 0
	for i := 0; i < count; i++ {
		for u := 0; u < len(temp_doses); u++ {
			for y := 0; y < len(temp_users); y++ {
				if ulogs[i].DrugName == test_drug &&
					ulogs[i].DrugRoute == test_route &&
					ulogs[i].DoseUnits == test_units &&
					ulogs[i].Dose == temp_doses[u] &&
					ulogs[i].Username == temp_users[y] &&
					ulogs != nil {
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

func useUser(i int, o int) int {
	if o == 0 {
		return 0
	}
	return i
}

func TestConcurrentGetLogs(t *testing.T) {
	for _, v := range testWithDrivers() {
		db, ctx, cfg := initForTests(v)
		defer db.Close()

		temp_doses := genLogDoses()
		temp_users := genLogUsers()

		for o := 0; o < 2; o++ {
			if o == 0 {
				fmt.Println("\t=== Testing the same username ===")
			}

			if o == 1 {
				fmt.Println("\t=== Testing different usernames ===")
			}

			count := 0
			for count < 5 {
				ret := cfg.AddToDoseDB(db, ctx, temp_users[useUser(count, o)], test_drug,
					test_route, temp_doses[count], test_units, 0, true)
				if ret == false {
					t.Fail()
					break
				}
				count++
			}
			logsChannel := make(chan []UserLog)

			for i := 0; i < count; i++ {
				go cfg.GetLogs(db, logsChannel, ctx, count, 0, temp_users[useUser(i, o)], true, "")
			}

			for i := 0; i < count; i++ {
				gotLog := <-logsChannel
				snd_count := count
				if o == 1 {
					snd_count = 1
				}
				if logIsInvalid(gotLog, temp_doses, temp_users, snd_count) {
					fmt.Println("\tFailed reading database, breaking.")
					t.Log("Didn't read database properly concurrently.")
					t.Fail()
					break
				}
			}

			for i := 0; i < count; i++ {
				cfg.RemoveLogs(db, ctx, temp_users[useUser(i, o)], 1, true, 0, "")
			}
		}
		cfg.cleanAfterTest(db, ctx)
	}
}
