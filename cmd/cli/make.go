package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
)

func doMake(arg2, arg3 string) error {

	switch arg2 {
	case "key":
		rnd := gem.RandomString(32)
		color.Green("Your new encryption key is: %s", rnd)

	case "auth":
		err := doAuth()
		if err != nil {
			exitGracefully(err)
		}
	case "mail":
		if arg3 == "" {
			exitGracefully(errors.New("you must give the mail a name"))
		}

		err := doMail(arg3)
		if err != nil {
			exitGracefully(err)
		}
	case "handler":
		if arg3 == "" {
			exitGracefully(errors.New("you must give the handler a name"))
		}

		fileName := gem.RootPath + "/handlers/" + strings.ToLower(arg3) + ".go"
		if fileExists(fileName) {
			exitGracefully(errors.New(fileName + " already exists."))
		}

		data, err := templateFS.ReadFile("templates/handlers/handler.go.txt")
		if err != nil {
			exitGracefully(err)
		}

		handler := string(data)
		handler = strings.ReplaceAll(handler, "$HANDLERNAME$", strcase.ToCamel(arg3))

		err = ioutil.WriteFile(fileName, []byte(handler), 0644)
		if err != nil {
			exitGracefully(err)
		}
	case "migration":

		if arg3 == "" {
			exitGracefully(errors.New("migration name is required"))
		}

		dbType := gem.DB.DataType
		fileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), arg3, dbType)

		upFile := gem.RootPath + "/migrations/" + fileName + ".up.sql"
		downFile := gem.RootPath + "/migrations/" + fileName + ".down.sql"

		err := copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", upFile)
		if err != nil {
			exitGracefully(err)
		}

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", downFile)
		if err != nil {
			exitGracefully(err)
		}
	case "model":
		if arg3 == "" {
			exitGracefully(errors.New("model name is required"))
		}

		data, err := templateFS.ReadFile("templates/data/model.go.txt")
		if err != nil {
			exitGracefully(err)
		}

		model := string(data)
		plural := pluralize.NewClient()
		var modelName = arg3
		var tableName = arg3

		if plural.IsPlural(arg3) {
			modelName = plural.Singular(arg3)
			tableName = strings.ToLower(arg3)
		} else {
			tableName = strings.ToLower(plural.Plural(arg3))
		}

		fileName := gem.RootPath + "/data/" + strings.ToLower(modelName) + ".go"
		if fileExists(fileName) {
			exitGracefully(errors.New(fileName + " already exists."))
		}

		model = strings.ReplaceAll(model, "$MODELNAME$", strcase.ToCamel(arg3))
		model = strings.ReplaceAll(model, "$TABLENAME$", tableName)

		err = copyDataToFile([]byte(model), fileName)
		if err != nil {
			exitGracefully(err)
		}

		color.Green("Model created: %s", fileName)

	case "session":
		err := doSession()
		if err != nil {
			exitGracefully(err)
		}

	default:
		exitGracefully(errors.New("Unknown subcommand" + arg3))
	}

	return nil
}
