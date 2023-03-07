package gemquick

import (
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func (g *Gemquick) MigrateUp(dsn string) error {
	m, err := migrate.New("file://"+g.RootPath+"/migrations", dsn)

	if err != nil {
		return err
	}

	defer m.Close()

	if err = m.Up(); err != nil {
		log.Println("Error while migrating up:", err)
		return err
	}

	return nil
}

func (g *Gemquick) MigrateDownAll(dsn string) error {
	m, err := migrate.New("file://"+g.RootPath+"/migrations", dsn)

	if err != nil {
		return err
	}

	defer m.Close()

	if err = m.Down(); err != nil {
		return err
	}

	return nil
}

func (g *Gemquick) Steps(steps int, dsn string) error {

	m, err := migrate.New("file://"+g.RootPath+"/migrations", dsn)

	if err != nil {
		return err
	}

	defer m.Close()

	if err = m.Steps(steps); err != nil {
		return err
	}

	return nil
}

func (g *Gemquick) MigrateForce(dsn string) error {
	m, err := migrate.New("file://"+g.RootPath+"/migrations", dsn)

	if err != nil {
		return err
	}

	defer m.Close()

	if err = m.Force(-1); err != nil {
		return err
	}

	return nil
}
