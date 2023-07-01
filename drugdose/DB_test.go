package drugdose

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

const test_drug string = "test_drug"
const test_route string = "test_route"
const test_units string = "test_units"
const test_user string = "test_user"
const test_source string = "test"

// TODO: Use later to test both drivers
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

func TestConcurrentGetLogs(t *testing.T) {
	// TODO: Use testWithDrivers() to test all drivers in a loop!
	db, ctx, cfg := initForTests("sqlite3")
	defer db.Close()

	ret := cfg.AddToDoseDB(db, ctx, test_user, test_drug,
		test_route, 123, test_units, 0, true)
	if ret == false {
		os.Exit(1)
	}

	logsChannel := make(chan []UserLog, 5)

	for i := 0; i < 5; i++ {
		go cfg.GetLogs(db, logsChannel, ctx, 1, 0, test_user, true, "")
	}

	for i := 0; i < 5; i++ {
		msg := <-logsChannel
		if msg[0].DrugName != test_drug || msg[0].DrugRoute != test_route ||
			msg[0].DoseUnits != test_units {
			t.Log("Didn't read database properly concurrently.")
			t.Fail()
			break
		}
	}

	cfg.RemoveLogs(db, ctx, test_user, 1, true, 0, "")
	cfg.cleanAfterTest(db, ctx)
}
