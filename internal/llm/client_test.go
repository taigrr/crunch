package llm

import (
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

func TestDetectProviderUsesRawProviderEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected Provider
	}{
		{
			name:     "anthropic",
			env:      map[string]string{"ANTHROPIC_API_KEY": "ant-test"},
			expected: ProviderAnthropic,
		},
		{
			name:     "openai",
			env:      map[string]string{"OPENAI_API_KEY": "openai-test"},
			expected: ProviderOpenAI,
		},
		{
			name:     "openrouter",
			env:      map[string]string{"OPENROUTER_API_KEY": "openrouter-test"},
			expected: ProviderOpenRouter,
		},
		{
			name:     "aws access keys",
			env:      map[string]string{"AWS_ACCESS_KEY_ID": "key", "AWS_SECRET_ACCESS_KEY": "secret"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws profile",
			env:      map[string]string{"AWS_PROFILE": "default"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws sso",
			env:      map[string]string{"AWS_SSO_SESSION": "session"},
			expected: ProviderBedrock,
		},
		{
			name:     "aws web identity",
			env:      map[string]string{"AWS_WEB_IDENTITY_TOKEN_FILE": "/tmp/token", "AWS_ROLE_ARN": "arn"},
			expected: ProviderBedrock,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearProviderEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			got, err := detectProvider()
			if err != nil {
				t.Fatalf("detectProvider() unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("detectProvider() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestAPIKeyFromEnvUsesRawProviderEnvVars(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "ant-test")
	t.Setenv("OPENAI_API_KEY", "openai-test")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-test")

	tests := []struct {
		provider Provider
		expected string
	}{
		{ProviderAnthropic, "ant-test"},
		{ProviderOpenAI, "openai-test"},
		{ProviderOpenRouter, "openrouter-test"},
		{ProviderBedrock, ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			got := apiKeyFromEnv(tc.provider)
			if got != tc.expected {
				t.Fatalf("apiKeyFromEnv(%q) = %q, want %q", tc.provider, got, tc.expected)
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

func clearProviderEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
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
	} {
		t.Setenv(key, "")
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
