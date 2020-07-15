package fiber

import (
	"os"
	"testing"
	"time"

	utils "github.com/gofiber/utils"
)

func Test_App_Prefork_Child_Process(t *testing.T) {
	utils.AssertEqual(t, nil, os.Setenv(envPreforkChildKey, envPreforkChildVal))
	defer os.Setenv(envPreforkChildKey, "")

	app := New(&Settings{
		DisableStartupMessage: true,
	})
	app.init()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("127.0.0.1:"))
}

func Test_App_Prefork_Main_Process(t *testing.T) {
	testPreforkMaster = true

	app := New(&Settings{
		DisableStartupMessage: true,
	})
	app.init()

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.prefork("127.0.0.1:"))
}
