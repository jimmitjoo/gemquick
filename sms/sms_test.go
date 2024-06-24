package sms

import (
	"errors"
	"log"
	"testing"
)

type MockSMSProvider struct {
	FromNumber string
}

func (m *MockSMSProvider) Send(to string, message string, unicode bool) error {
	m.FromNumber = "0123456789"

	if unicode {
		log.Println("Sending unicode message")
	}

	if to == "" {
		return errors.New("A phone number is required")
	}

	if message == "" {
		return errors.New("A message is required")
	}

	return nil
}

func TestSendSMS(t *testing.T) {
	mockProvider := &MockSMSProvider{}

	to := "1234567890"
	message := "Test message"

	// Assume we have a function Send that uses an SMSProvider to send an SMS
	err := mockProvider.Send(to, message, false)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	err = mockProvider.Send("", message, false)
	if err == nil {
		t.Errorf("Expected an error, but got nil")
	}

	err = mockProvider.Send(to, "", false)
	if err == nil {
		t.Errorf("Expected an error, but got nil")
	}

	if mockProvider.FromNumber != "0123456789" {
		t.Errorf("Expected to = %v, but got: %v", to, "0123456789")
	}
}
