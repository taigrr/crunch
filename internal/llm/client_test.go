package llm

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseProvider(t *testing.T) {
	tests := []struct {
		input    string
		expected Provider
		wantErr  bool
	}{
		{"bedrock", ProviderBedrock, false},
		{"aws", ProviderBedrock, false},
		{"anthropic", ProviderAnthropic, false},
		{"claude", ProviderAnthropic, false},
		{"openai", ProviderOpenAI, false},
		{"gpt", ProviderOpenAI, false},
		{"openrouter", ProviderOpenRouter, false},
		{"BEDROCK", ProviderBedrock, false},
		{" openai ", ProviderOpenAI, false},
		{"\tclaude\n", ProviderAnthropic, false},
		{"unknown", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseProvider(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseProvider(%q) expected error", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseProvider(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.expected {
				t.Errorf("ParseProvider(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDefaultModelForProvider(t *testing.T) {
	tests := []struct {
		provider Provider
		expected string
	}{
		{ProviderBedrock, "us.anthropic.claude-sonnet-4-20250514-v1:0"},
		{ProviderAnthropic, "claude-sonnet-4-20250514"},
		{ProviderOpenAI, "gpt-4o"},
		{ProviderOpenRouter, "anthropic/claude-sonnet-4"},
		{"", "us.anthropic.claude-sonnet-4-20250514-v1:0"},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			got := defaultModelForProvider(tc.provider)
			if got != tc.expected {
				t.Errorf("defaultModelForProvider(%q) = %q, want %q", tc.provider, got, tc.expected)
			}
		})
	}
}

func TestBedrockToAnthropicModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "regional bedrock model",
			input:    "us.anthropic.claude-sonnet-4-20250514-v1:0",
			expected: "claude-sonnet-4-20250514",
		},
		{
			name:     "global bedrock model",
			input:    "anthropic.claude-3-5-haiku-20241022-v1:0",
			expected: "claude-3-5-haiku-20241022",
		},
		{
			name:     "already anthropic model",
			input:    "claude-sonnet-4-20250514",
			expected: "claude-sonnet-4-20250514",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bedrockToAnthropicModel(tc.input)
			if got != tc.expected {
				t.Errorf("bedrockToAnthropicModel(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	client := &Client{
		pricing: Pricing{
			CostPer1MIn:  3,
			CostPer1MOut: 15,
		},
	}

	got := client.CalculateCost(1_000_000, 2_000_000)
	const expected = 33.0
	if got != expected {
		t.Errorf("CalculateCost() = %f, want %f", got, expected)
	}
}

func TestClientGetters(t *testing.T) {
	client := &Client{
		pricing:  Pricing{CostPer1MIn: 1.5, CostPer1MOut: 7.5},
		provider: ProviderOpenRouter,
		model:    "anthropic/claude-sonnet-4",
	}

	if got := client.GetPricing(); got != client.pricing {
		t.Fatalf("GetPricing() = %+v, want %+v", got, client.pricing)
	}
	if got := client.GetProvider(); got != ProviderOpenRouter {
		t.Fatalf("GetProvider() = %q, want %q", got, ProviderOpenRouter)
	}
	if got := client.GetModel(); got != "anthropic/claude-sonnet-4" {
		t.Fatalf("GetModel() = %q", got)
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected Provider
		wantErr  error
	}{
		{
			name:     "crunch api key defaults to anthropic",
			env:      map[string]string{"CRUNCH_API_KEY": "sk-crunch"},
			expected: ProviderAnthropic,
		},
		{
			name:     "anthropic key",
			env:      map[string]string{"ANTHROPIC_API_KEY": "sk-ant"},
			expected: ProviderAnthropic,
		},
		{
			name:     "openai key",
			env:      map[string]string{"OPENAI_API_KEY": "sk-openai"},
			expected: ProviderOpenAI,
		},
		{
			name:     "openrouter key",
			env:      map[string]string{"OPENROUTER_API_KEY": "sk-or"},
			expected: ProviderOpenRouter,
		},
		{
			name:     "aws static credentials",
			env:      map[string]string{"AWS_ACCESS_KEY_ID": "AKIA", "AWS_SECRET_ACCESS_KEY": "secret"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws profile",
			env:      map[string]string{"AWS_PROFILE": "work"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws sso session",
			env:      map[string]string{"AWS_SSO_SESSION": "work"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws web identity",
			env:      map[string]string{"AWS_WEB_IDENTITY_TOKEN_FILE": "/tmp/token", "AWS_ROLE_ARN": "arn:aws:iam::123:role/test"},
			expected: ProviderBedrock,
		},
		{
			name:    "no credentials",
			wantErr: ErrNoCredentials,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetCredentialEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			got, err := detectProvider()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("detectProvider() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectProvider() unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("detectProvider() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestDetectProviderPrecedence(t *testing.T) {
	resetCredentialEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-or")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")

	got, err := detectProvider()
	if err != nil {
		t.Fatal(err)
	}
	if got != ProviderAnthropic {
		t.Fatalf("detectProvider() = %q, want %q", got, ProviderAnthropic)
	}
}

func TestAPIKeyFromEnv(t *testing.T) {
	resetCredentialEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("OPENROUTER_API_KEY", "sk-or")

	tests := []struct {
		provider Provider
		want     string
	}{
		{ProviderAnthropic, "sk-ant"},
		{ProviderOpenAI, "sk-openai"},
		{ProviderOpenRouter, "sk-or"},
		{ProviderBedrock, ""},
		{"unknown", ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			if got := apiKeyFromEnv(tc.provider); got != tc.want {
				t.Fatalf("apiKeyFromEnv(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

func TestCreateProviderMissingAPIKey(t *testing.T) {
	tests := []struct {
		provider Provider
		want     string
	}{
		{ProviderAnthropic, "ANTHROPIC_API_KEY not set"},
		{ProviderOpenAI, "OPENAI_API_KEY not set"},
		{ProviderOpenRouter, "OPENROUTER_API_KEY not set"},
		{"unknown", "unknown provider: unknown"},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			_, err := createProvider(tc.provider, "")
			if err == nil {
				t.Fatal("createProvider() expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("createProvider() error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) != 4 {
		t.Errorf("SupportedProviders() returned %d providers, want 4", len(providers))
	}
}

func TestParseProviderErrorListsSupportedProviders(t *testing.T) {
	_, err := ParseProvider("unknown")
	if err == nil {
		t.Fatal("ParseProvider() expected error")
	}

	for _, provider := range SupportedProviders() {
		if !strings.Contains(err.Error(), provider) {
			t.Errorf("ParseProvider() error %q missing supported provider %q", err.Error(), provider)
		}
	}
}

func resetCredentialEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"CRUNCH_API_KEY",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"OPENROUTER_API_KEY",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_PROFILE",
		"AWS_SSO_SESSION",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
		"AWS_ROLE_ARN",
	}
	original := make(map[string]*string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			valueCopy := value
			original[key] = &valueCopy
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for _, key := range keys {
			value, ok := original[key]
			if !ok {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *value)
		}
	})
}
