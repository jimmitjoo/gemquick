package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
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

		// check if there is a database connection
		if gem.DB.DataType == "" {
			return errors.New("you have to define a database type to create migrations")
		}

		dbType := gem.DB.DataType
		fileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), arg3, dbType)

		upFile := gem.RootPath + "/migrations/" + fileName + ".up.sql"
		downFile := gem.RootPath + "/migrations/" + fileName + ".down.sql"

		err := copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", upFile)
		if err != nil {
			exitGracefully(err)
		}

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", downFile)
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

		modelCamelName := strcase.ToCamel(arg3)
		modelCamelNamePlural := pluralize.NewClient().Plural(modelCamelName)
		model = strings.ReplaceAll(model, "$MODELNAME$", modelCamelName)
		model = strings.ReplaceAll(model, "$TABLENAME$", tableName)

		err = copyDataToFile([]byte(model), fileName)
		if err != nil {
			exitGracefully(err)
		}

		color.Green(modelCamelName+" created: %s", fileName)

		// create a migration for the model
		dbType := gem.DB.DataType
		migrationFileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), "create_"+tableName+"_table", dbType)

		upFile := gem.RootPath + "/migrations/" + migrationFileName + ".up.sql"
		downFile := gem.RootPath + "/migrations/" + migrationFileName + ".down.sql"

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", upFile)
		if err != nil {
			exitGracefully(err)
		}

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", downFile)
		if err != nil {
			exitGracefully(err)
		}

		color.Green("Migrations for model %s created: %s", modelCamelName, migrationFileName)

		// add model to models.go
		modelsContent, err := os.ReadFile(gem.RootPath + "/data/models.go")
		if err != nil {
			exitGracefully(err)
		}

		if bytes.Contains(modelsContent, []byte(modelCamelName)) {
			exitGracefully(errors.New(modelCamelName + " already exists in models.go"))
		} else {
			modelsContent = bytes.Replace(modelsContent, []byte("type Models struct {"), []byte("type Models struct {\n\t"+modelCamelNamePlural+" "+modelCamelName+"\n"), 1)
			modelsContent = bytes.Replace(modelsContent, []byte("return Models{"), []byte("return Models{\n\t\t"+modelCamelNamePlural+": "+modelCamelName+"{},\n"), 1)
			if err = os.WriteFile(gem.RootPath+"/data/models.go", modelsContent, 0644); err != nil {
				exitGracefully(err)
			}

			color.Green(modelCamelName + " added to models.go")
		}

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
