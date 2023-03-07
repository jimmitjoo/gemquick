package email

import "testing"

func TestMail_SendSMTPMessage(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	err := mailer.SendSMTPMessage(msg)
	if err != nil {
		t.Error(err)
	}
}

func TestMail_SendUsingChan(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	mailer.Jobs <- msg
	result := <-mailer.Results
	if !result.Success {
		t.Error(result.Error)
	}

	msg.To = "not_a_valid_email"
	mailer.Jobs <- msg
	result = <-mailer.Results
	if result.Success {
		t.Error("no error received with invalid To address")
	}
}

func TestMail_SendUsingAPI(t *testing.T) {
	msg := Message{
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	mailer.API = "non_existent_api"
	mailer.APIKey = "no_valid_api_key"
	mailer.APIUrl = "https://www.fakeurl.com"

	err := mailer.SendUsingAPI(msg, "unknown_api")
	if err == nil {
		t.Error("no error received with invalid API")
	}

	mailer.API = ""
	mailer.APIKey = ""
	mailer.APIUrl = ""
}

func TestMail_BuildHTMLMessage(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	_, err := mailer.buildHTMLMessage(msg)
	if err != nil {
		t.Error(err)
	}
}

func TestMail_BuildPlainTextMessage(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	_, err := mailer.buildPlainTextMessage(msg)
	if err != nil {
		t.Error(err)
	}
}

func TestMail_send(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	err := mailer.Send(msg)
	if err != nil {
		t.Error(err)
	}

	mailer.API = "non_existent_api"
	mailer.APIKey = "no_valid_api_key"
	mailer.APIUrl = "https://www.fakeurl.com"

	err = mailer.Send(msg)
	if err == nil {
		t.Error("no error received with invalid API credentials")
	}

	mailer.API = ""
	mailer.APIKey = ""
	mailer.APIUrl = ""
}

func TestMail_ChooseAPI(t *testing.T) {
	msg := Message{
		From:        "test@test.com",
		FromName:    "Test",
		To:          "to@test.com",
		Subject:     "Test",
		Template:    "test",
		Attachments: []string{"testdata/email/test.plain.tmpl"},
	}

	mailer.API = "non_existent_api"

	err := mailer.ChooseAPI(msg)
	if err == nil {
		t.Error("no error received with invalid API")
	}
}
