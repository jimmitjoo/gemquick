package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
)

var appUrl string

func doNew(appName string) error {
	appname := strings.ToLower(appName)
	appUrl = appname

	// Sanitize the new application name
	if strings.Contains(appname, "/") {
		exploded := strings.SplitAfter(appname, "/")
		appname = exploded[len(exploded)-1]
	}

	log.Println("Creating new application: " + appname)

	// Git clone the skeleton application
	color.Green("\tCloning skeleton application...")
	_, err := git.PlainClone("./"+appname, false, &git.CloneOptions{
		URL:      "https://github.com/jimmitjoo/gemquick-bare.git",
		Progress: os.Stdout,
		Depth:    1,
	})
	if err != nil {
		exitGracefully(err)
	}

	// Remove .git directory
	err = os.RemoveAll("./" + appname + "/.git")
	if err != nil {
		exitGracefully(err)
	}

	// Create a ready to go .env file
	color.Green("\tCreating .env file...")
	data, err := templateFS.ReadFile("templates/env.txt")
	if err != nil {
		exitGracefully(err)
	}

	env := string(data)
	env = strings.ReplaceAll(env, "${APP_NAME}", appname)
	env = strings.ReplaceAll(env, "${KEY}", gem.RandomString(32))

	err = copyDataToFile([]byte(env), "./"+appname+"/.env")
	if err != nil {
		exitGracefully(err)
	}

	// Create a Makefile
	color.Green("\tCreating Makefile...")

	if runtime.GOOS == "windows" {
		source, err := os.Open(fmt.Sprintf("./%s/Makefile.windows", appname))
		if err != nil {
			exitGracefully(err)
		}
		defer source.Close()

		destination, err := os.Create(fmt.Sprintf("./%s/Makefile", appname))
		if err != nil {
			exitGracefully(err)
		}
		defer destination.Close()

		_, err = io.Copy(destination, source)
		if err != nil {
			exitGracefully(err)
		}

	} else {
		source, err := os.Open(fmt.Sprintf("./%s/Makefile.mac", appname))
		if err != nil {
			exitGracefully(err)
		}
		defer source.Close()

		destination, err := os.Create(fmt.Sprintf("./%s/Makefile", appname))
		if err != nil {
			exitGracefully(err)
		}
		defer destination.Close()

		_, err = io.Copy(destination, source)
		if err != nil {
			exitGracefully(err)
		}
	}

	os.Remove(fmt.Sprintf("./%s/Makefile.windows", appname))
	os.Remove(fmt.Sprintf("./%s/Makefile.mac", appname))

	// Update the go.mod file
	color.Green("\tCreating go.mod file...")
	os.Remove(fmt.Sprintf("./%s/go.mod", appname))

	data, err = templateFS.ReadFile("templates/go.mod.txt")
	if err != nil {
		exitGracefully(err)
	}

	gomod := string(data)
	gomod = strings.ReplaceAll(gomod, "${APP_NAME}", appname)

	err = copyDataToFile([]byte(gomod), "./"+appname+"/go.mod")
	if err != nil {
		exitGracefully(err)
	}

	// Update the existing .go files with correct name/imports
	color.Green("\tUpdating source files...")
	os.Chdir("./" + appname)
	updateSource()

	// Run go mod tidy
	color.Green("\tRunning go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	err = cmd.Start()
	if err != nil {
		exitGracefully(err)
	}

	color.Green("\tDone building " + appname + "!")
	color.Green("\tGo build something great!")

	return nil
}
