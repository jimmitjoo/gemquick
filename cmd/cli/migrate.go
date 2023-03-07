package main

func doMigrate(arg2, arg3 string) error {
	dsn := getDSN()

	switch arg2 {
	case "up":
		err := gem.MigrateUp(dsn)
		if err != nil {
			return err
		}
	case "down":
		if arg3 == "all" {
			err := gem.MigrateDownAll(dsn)
			if err != nil {
				return err
			}
			return nil
		} else {
			err := gem.Steps(-1, dsn)
			if err != nil {
				return err
			}
			return nil
		}
	case "reset":
		err := gem.MigrateDownAll(dsn)
		if err != nil {
			return err
		}
		err = gem.MigrateUp(dsn)
		if err != nil {
			return err
		}

	default:
		showHelp()
	}
	return nil
}
