package main

import "strings"

func doMail(arg3 string) error {
	htmlMail := gem.RootPath + "/email/" + strings.ToLower(arg3) + ".html.tmpl"
	plainTextMail := gem.RootPath + "/email/" + strings.ToLower(arg3) + ".plain.tmpl"

	err := copyFileFromTemplate("templates/email/html.tmpl.txt", htmlMail)
	if err != nil {
		return err
	}

	err = copyFileFromTemplate("templates/email/plain.tmpl.txt", plainTextMail)
	if err != nil {
		return err
	}

	return nil
}
