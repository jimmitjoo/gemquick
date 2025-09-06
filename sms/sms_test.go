package sms

import (
	"errors"
	"os"
	"testing"
)

// MockSMSProvider for testing the interface
type MockSMSProvider struct {
	FromNumber   string
	SendCalled   bool
	SendError    error
	LastTo       string
	LastMessage  string
	LastUnicode  bool
}

func (m *MockSMSProvider) Send(to string, message string, unicode bool) error {
	m.SendCalled = true
	m.LastTo = to
	m.LastMessage = message
	m.LastUnicode = unicode
	
	if m.SendError != nil {
		return m.SendError
	}
	
	if to == "" {
		return errors.New("phone number is required")
	}
	
	if message == "" {
		return errors.New("message is required")
	}
	
	return nil
}

func TestMockSMSProvider_Send(t *testing.T) {
	tests := []struct {
		name        string
		to          string
		message     string
		unicode     bool
		sendError   error
		expectError bool
	}{
		{
			name:        "Valid SMS",
			to:          "+1234567890",
			message:     "Test message",
			unicode:     false,
			sendError:   nil,
			expectError: false,
		},
		{
			name:        "Valid SMS with Unicode",
			to:          "+1234567890",
			message:     "Test message with emoji 😀",
			unicode:     true,
			sendError:   nil,
			expectError: false,
		},
		{
			name:        "Empty phone number",
			to:          "",
			message:     "Test message",
			unicode:     false,
			sendError:   nil,
			expectError: true,
		},
		{
			name:        "Empty message",
			to:          "+1234567890",
			message:     "",
			unicode:     false,
			sendError:   nil,
			expectError: true,
		},
		{
			name:        "Provider error",
			to:          "+1234567890",
			message:     "Test message",
			unicode:     false,
			sendError:   errors.New("provider error"),
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockSMSProvider{
				SendError: tt.sendError,
			}
			
			err := mock.Send(tt.to, tt.message, tt.unicode)
			
			if !mock.SendCalled {
				t.Error("Expected Send to be called")
			}
			
			if mock.LastTo != tt.to {
				t.Errorf("Expected to = %s, got %s", tt.to, mock.LastTo)
			}
			
			if mock.LastMessage != tt.message {
				t.Errorf("Expected message = %s, got %s", tt.message, mock.LastMessage)
			}
			
			if mock.LastUnicode != tt.unicode {
				t.Errorf("Expected unicode = %v, got %v", tt.unicode, mock.LastUnicode)
			}
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestVonage_New(t *testing.T) {
	// Set test environment variables
	os.Setenv("VONAGE_API_KEY", "test-key")
	os.Setenv("VONAGE_API_SECRET", "test-secret")
	os.Setenv("VONAGE_FROM_NUMBER", "+1234567890")
	defer func() {
		os.Unsetenv("VONAGE_API_KEY")
		os.Unsetenv("VONAGE_API_SECRET")
		os.Unsetenv("VONAGE_FROM_NUMBER")
	}()
	
	provider := CreateSMSProvider("vonage")
	
	vonage, ok := provider.(*Vonage)
	if !ok {
		t.Fatal("Expected Vonage provider")
	}
	
	if vonage.APIKey != "test-key" {
		t.Errorf("Expected APIKey = test-key, got %s", vonage.APIKey)
	}
	
	if vonage.APISecret != "test-secret" {
		t.Errorf("Expected APISecret = test-secret, got %s", vonage.APISecret)
	}
	
	if vonage.FromNumber != "+1234567890" {
		t.Errorf("Expected FromNumber = +1234567890, got %s", vonage.FromNumber)
	}
}

func TestTwilio_New(t *testing.T) {
	// Set test environment variables
	os.Setenv("TWILIO_ACCOUNT_SID", "test-sid")
	os.Setenv("TWILIO_API_KEY", "test-key")
	os.Setenv("TWILIO_API_SECRET", "test-secret")
	os.Setenv("TWILIO_FROM_NUMBER", "+0987654321")
	defer func() {
		os.Unsetenv("TWILIO_ACCOUNT_SID")
		os.Unsetenv("TWILIO_API_KEY")
		os.Unsetenv("TWILIO_API_SECRET")
		os.Unsetenv("TWILIO_FROM_NUMBER")
	}()
	
	provider := CreateSMSProvider("twilio")
	
	twilio, ok := provider.(*Twilio)
	if !ok {
		t.Fatal("Expected Twilio provider")
	}
	
	if twilio.AccountSid != "test-sid" {
		t.Errorf("Expected AccountSid = test-sid, got %s", twilio.AccountSid)
	}
	
	if twilio.APIKey != "test-key" {
		t.Errorf("Expected APIKey = test-key, got %s", twilio.APIKey)
	}
	
	if twilio.APISecret != "test-secret" {
		t.Errorf("Expected APISecret = test-secret, got %s", twilio.APISecret)
	}
	
	if twilio.FromNumber != "+0987654321" {
		t.Errorf("Expected FromNumber = +0987654321, got %s", twilio.FromNumber)
	}
}

func TestCreateSMSProvider(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		expectNil    bool
		expectedType string
	}{
		{
			name:         "Vonage provider",
			provider:     "vonage",
			expectNil:    false,
			expectedType: "*sms.Vonage",
		},
		{
			name:         "Twilio provider",
			provider:     "twilio",
			expectNil:    false,
			expectedType: "*sms.Twilio",
		},
		{
			name:         "Unknown provider",
			provider:     "unknown",
			expectNil:    true,
			expectedType: "",
		},
		{
			name:         "Empty provider",
			provider:     "",
			expectNil:    true,
			expectedType: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := CreateSMSProvider(tt.provider)
			
			if tt.expectNil && provider != nil {
				t.Errorf("Expected nil provider for %s", tt.provider)
			}
			
			if !tt.expectNil && provider == nil {
				t.Errorf("Expected non-nil provider for %s", tt.provider)
			}
		})
	}
}

func TestSMSProviderInterface(t *testing.T) {
	// Ensure both Vonage and Twilio implement SMSProvider
	var _ SMSProvider = (*Vonage)(nil)
	var _ SMSProvider = (*Twilio)(nil)
	var _ SMSProvider = (*MockSMSProvider)(nil)
}

// Benchmark tests
func BenchmarkMockSMSProvider_Send(b *testing.B) {
	mock := &MockSMSProvider{}
	to := "+1234567890"
	message := "Test message"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mock.Send(to, message, false)
	}
}

func BenchmarkCreateSMSProvider_Vonage(b *testing.B) {
	os.Setenv("VONAGE_API_KEY", "test")
	os.Setenv("VONAGE_API_SECRET", "test")
	os.Setenv("VONAGE_FROM_NUMBER", "test")
	defer func() {
		os.Unsetenv("VONAGE_API_KEY")
		os.Unsetenv("VONAGE_API_SECRET")
		os.Unsetenv("VONAGE_FROM_NUMBER")
	}()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateSMSProvider("vonage")
	}
}

func BenchmarkCreateSMSProvider_Twilio(b *testing.B) {
	os.Setenv("TWILIO_ACCOUNT_SID", "test")
	os.Setenv("TWILIO_API_KEY", "test")
	os.Setenv("TWILIO_API_SECRET", "test")
	os.Setenv("TWILIO_FROM_NUMBER", "test")
	defer func() {
		os.Unsetenv("TWILIO_ACCOUNT_SID")
		os.Unsetenv("TWILIO_API_KEY")
		os.Unsetenv("TWILIO_API_SECRET")
		os.Unsetenv("TWILIO_FROM_NUMBER")
	}()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CreateSMSProvider("twilio")
	}
}