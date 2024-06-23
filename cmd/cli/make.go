package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gertd/go-pluralize"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/iancoleman/strcase"
)

func doMake(arg2, arg3 string) error {

	switch arg2 {
	case "key":
		handleKey()

	case "auth":
		handleAuth()

	case "mail":
		handleMail(arg3)

	case "handler":
		handleHandler(arg3)

	case "migration":
		handleMigration(arg3)

	case "model":
		handleModel(arg3)

	case "session":
		handleSession()

	default:
		exitGracefully(errors.New("Unknown subcommand" + arg3))
	}

	return nil
}

func handleKey() {
	rnd := gem.RandomString(32)
	color.Green("Your new encryption key is: %s", rnd)
}

func handleAuth() {
	err := doAuth()
	if err != nil {
		exitGracefully(err)
	}
}

func handleMail(name string) {
	if name == "" {
		exitGracefully(errors.New("you must give the mail a name"))
	}

	err := doMail(name)
	if err != nil {
		exitGracefully(err)
	}
}

func handleHandler(name string) {
	if name == "" {
		exitGracefully(errors.New("you must give the handler a name"))
	}

	fileName := gem.RootPath + "/handlers/" + strings.ToLower(name) + ".go"
	if fileExists(fileName) {
		exitGracefully(errors.New(fileName + " already exists."))
	}

	data, err := templateFS.ReadFile("templates/handlers/handler.go.txt")
	if err != nil {
		exitGracefully(err)
	}

	handler := string(data)
	handler = strings.ReplaceAll(handler, "$HANDLERNAME$", strcase.ToCamel(name))

	err = ioutil.WriteFile(fileName, []byte(handler), 0644)
	if err != nil {
		exitGracefully(err)
	}
}

func handleMigration(name string) {
	if name == "" {
		exitGracefully(errors.New("migration name is required"))
	}

	// check if there is a database connection
	if gem.DB.DataType == "" {
		panic("you have to define a database type to create migrations")
	}

	dbType := gem.DB.DataType
	fileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), name, dbType)

	migrationUpFile := gem.RootPath + "/migrations/" + fileName + ".up.sql"
	migrationDownFile := gem.RootPath + "/migrations/" + fileName + ".down.sql"

	err := copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", migrationUpFile)
	if err != nil {
		exitGracefully(err)
	}

	err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", migrationDownFile)
	if err != nil {
		exitGracefully(err)
	}

	reformatMigration(migrationUpFile, name)
	reformatMigration(migrationDownFile, name)
}

func reformatMigration(migrationFile string, tableName string) {
	content, err := os.ReadFile(migrationFile)
	if err != nil {
		exitGracefully(err)
	}

	if bytes.Contains(content, []byte("TABLENAME")) {
		content = bytes.ReplaceAll(content, []byte("TABLENAME"), []byte(tableName))
		if err = os.WriteFile(migrationFile, content, 0644); err != nil {
			exitGracefully(err)
		}
	}
}

func handleModel(name string) {
	if name == "" {
		exitGracefully(errors.New("model name is required"))
	}

	data, err := templateFS.ReadFile("templates/data/model.go.txt")
	if err != nil {
		exitGracefully(err)
	}

	model := string(data)
	plural := pluralize.NewClient()
	var modelName = name
	var tableName = name

	if plural.IsPlural(name) {
		modelName = plural.Singular(name)
		tableName = strings.ToLower(name)
	} else {
		tableName = strings.ToLower(plural.Plural(name))
	}

	fileName := gem.RootPath + "/data/" + strings.ToLower(modelName) + ".go"
	if fileExists(fileName) {
		exitGracefully(errors.New(fileName + " already exists."))
	}

	modelCamelName := strcase.ToCamel(name)
	modelCamelNamePlural := pluralize.NewClient().Plural(modelCamelName)
	model = strings.ReplaceAll(model, "$MODELNAME$", modelCamelName)
	model = strings.ReplaceAll(model, "$TABLENAME$", tableName)

	err = copyDataToFile([]byte(model), fileName)
	if err != nil {
		exitGracefully(err)
	}

	color.Green(modelCamelName+" created: %s", fileName)

	// create a migration for the model
	if gem.DB.DataType != "" {

		dbType := gem.DB.DataType
		migrationFileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), "create_"+tableName+"_table", dbType)

		migrationUpFile := gem.RootPath + "/migrations/" + migrationFileName + ".up.sql"
		migrationDownFile := gem.RootPath + "/migrations/" + migrationFileName + ".down.sql"

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", migrationUpFile)
		if err != nil {
			exitGracefully(err)
		}

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", migrationDownFile)
		if err != nil {
			exitGracefully(err)
		}

		reformatMigration(migrationUpFile, tableName)
		reformatMigration(migrationDownFile, tableName)

		color.Green("Migrations for model %s created: %s", modelCamelName, migrationFileName)

		// add model to models.go
		modelsContent, err := os.ReadFile(gem.RootPath + "/data/models.go")
		if err != nil {
			exitGracefully(err)
		}

		// replace stubfile with the new model
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
	}
}

func handleSession() {
	err := doSession()
	if err != nil {
		exitGracefully(err)
	}
}
