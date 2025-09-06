package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gertd/go-pluralize"
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
		return doAuth()

	case "mail":
		return doMail(arg3)

	case "handler":
		return doHandler(arg3)

	case "migration":
		return doMigration(arg3)

	case "model":
		return doModel(arg3)

	case "session":
		return doSession()

	default:
		return errors.New("Unknown subcommand " + arg2)
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
	err := doHandler(name)
	if err != nil {
		exitGracefully(err)
	}
}

func doHandler(name string) error {
	if name == "" {
		return errors.New("you must give the handler a name")
	}

	fileName := gem.RootPath + "/handlers/" + strings.ToLower(name) + ".go"
	if fileExists(fileName) {
		return errors.New(fileName + " already exists.")
	}

	data, err := templateFS.ReadFile("templates/handlers/handler.go.txt")
	if err != nil {
		return err
	}

	handler := string(data)
	handler = strings.ReplaceAll(handler, "$HANDLERNAME$", strcase.ToCamel(name))

	err = os.WriteFile(fileName, []byte(handler), 0644)
	if err != nil {
		return err
	}
	
	return nil
}

func handleMigration(name string) {
	err := doMigration(name)
	if err != nil {
		exitGracefully(err)
	}
}

func doMigration(name string) error {
	if name == "" {
		return errors.New("migration name is required")
	}

	// check if there is a database connection
	if gem.DB.DataType == "" {
		return errors.New("you have to define a database type to create migrations")
	}

	dbType := gem.DB.DataType
	fileName := fmt.Sprintf("%d_%s.%s", time.Now().UnixMicro(), name, dbType)

	migrationUpFile := gem.RootPath + "/migrations/" + fileName + ".up.sql"
	migrationDownFile := gem.RootPath + "/migrations/" + fileName + ".down.sql"

	err := copyFileFromTemplate("templates/migrations/migration."+dbType+".up.sql", migrationUpFile)
	if err != nil {
		return err
	}

	err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", migrationDownFile)
	if err != nil {
		return err
	}

	err = reformatMigration(migrationUpFile, name)
	if err != nil {
		return err
	}
	
	err = reformatMigration(migrationDownFile, name)
	if err != nil {
		return err
	}
	
	return nil
}

func reformatMigration(migrationFile string, tableName string) error {
	content, err := os.ReadFile(migrationFile)
	if err != nil {
		return err
	}

	if bytes.Contains(content, []byte("TABLENAME")) {
		content = bytes.ReplaceAll(content, []byte("TABLENAME"), []byte(tableName))
		if err = os.WriteFile(migrationFile, content, 0644); err != nil {
			return err
		}
	}
	
	return nil
}

func handleModel(name string) {
	err := doModel(name)
	if err != nil {
		exitGracefully(err)
	}
}

func doModel(name string) error {
	if name == "" {
		return errors.New("model name is required")
	}

	data, err := templateFS.ReadFile("templates/data/model.go.txt")
	if err != nil {
		return err
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
		return errors.New(fileName + " already exists.")
	}

	modelCamelName := strcase.ToCamel(name)
	modelCamelNamePlural := pluralize.NewClient().Plural(modelCamelName)
	model = strings.ReplaceAll(model, "$MODELNAME$", modelCamelName)
	model = strings.ReplaceAll(model, "$TABLENAME$", tableName)

	err = copyDataToFile([]byte(model), fileName)
	if err != nil {
		return err
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
			return err
		}

		err = copyFileFromTemplate("templates/migrations/migration."+dbType+".down.sql", migrationDownFile)
		if err != nil {
			return err
		}

		err = reformatMigration(migrationUpFile, tableName)
		if err != nil {
			return err
		}
		
		err = reformatMigration(migrationDownFile, tableName)
		if err != nil {
			return err
		}

		color.Green("Migrations for model %s created: %s", modelCamelName, migrationFileName)

		// add model to models.go
		modelsContent, err := os.ReadFile(gem.RootPath + "/data/models.go")
		if err != nil {
			return err
		}

		// replace stubfile with the new model
		if bytes.Contains(modelsContent, []byte(modelCamelName)) {
			return errors.New(modelCamelName + " already exists in models.go")
		} else {
			modelsContent = bytes.Replace(modelsContent, []byte("type Models struct {"), []byte("type Models struct {\n\t"+modelCamelNamePlural+" "+modelCamelName+"\n"), 1)
			modelsContent = bytes.Replace(modelsContent, []byte("return Models{"), []byte("return Models{\n\t\t"+modelCamelNamePlural+": "+modelCamelName+"{},\n"), 1)
			if err = os.WriteFile(gem.RootPath+"/data/models.go", modelsContent, 0644); err != nil {
				return err
			}

			color.Green(modelCamelName + " added to models.go")
		}
	}
	
	return nil
}

func handleSession() {
	err := doSession()
	if err != nil {
		exitGracefully(err)
	}
}
