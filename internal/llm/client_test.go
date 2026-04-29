package llm

import (
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
		contains string
	}{
		{ProviderBedrock, "claude"},
		{ProviderAnthropic, "claude"},
		{ProviderOpenAI, "gpt"},
		{ProviderOpenRouter, "claude"},
	}

	for _, tc := range tests {
		t.Run(string(tc.provider), func(t *testing.T) {
			got := defaultModelForProvider(tc.provider)
			if got == "" {
				t.Errorf("defaultModelForProvider(%q) returned empty string", tc.provider)
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
