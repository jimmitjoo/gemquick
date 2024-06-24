package sms

import (
	"errors"
	"fmt"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
	"github.com/vonage/vonage-go-sdk"
	"os"
)

// SMSProvider SMS is an interface that defines the methods that an SMS provider must implement
type SMSProvider interface {
	Send(to string, message string, unicode bool) error
}

type Vonage struct {
	APIKey     string
	APISecret  string
	FromNumber string
}

type Twilio struct {
	AccountSid string
	APIKey     string
	APISecret  string
	FromNumber string
}

func (v *Vonage) Send(to string, msg string, unicode bool) error {
	auth := vonage.CreateAuthFromKeySecret(v.APIKey, v.APISecret)
	client := vonage.NewSMSClient(auth)

	smsOpts := vonage.SMSOpts{}
	if unicode {
		smsOpts.Type = "unicode"
	}

	response, _, err := client.Send(v.FromNumber, to, msg, smsOpts)
	if err != nil {
		return err
	}
	if response.Messages[0].Status != "0" {
		return errors.New(response.Messages[0].Status)
	}

	return nil
}

func (t *Twilio) Send(to string, msg string, unicode bool) error {

	// Tell the user that Twilio always sends messages in unicode
	if unicode {
		fmt.Println("Twilio always sends messages in unicode")
	}

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username:   t.APIKey,
		Password:   t.APISecret,
		AccountSid: t.AccountSid,
	})

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(to)
	params.SetFrom(t.FromNumber)
	params.SetBody(msg)

	_, err := client.Api.CreateMessage(params)
	if err != nil {
		fmt.Println("Error sending SMS message: " + err.Error())
		return err
	}

	return nil
}

func CreateSMSProvider(provider string) SMSProvider {
	switch provider {
	case "vonage":
		return &Vonage{
			APIKey:     os.Getenv("VONAGE_API_KEY"),
			APISecret:  os.Getenv("VONAGE_API_SECRET"),
			FromNumber: os.Getenv("VONAGE_FROM_NUMBER"),
		}
	case "twilio":
		return &Twilio{
			AccountSid: os.Getenv("TWILIO_ACCOUNT_SID"),
			APIKey:     os.Getenv("TWILIO_API_KEY"),
			APISecret:  os.Getenv("TWILIO_API_SECRET"),
			FromNumber: os.Getenv("TWILIO_FROM_NUMBER"),
		}
	default:
		return nil
	}
}
