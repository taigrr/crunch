// Package llm provides a multi-provider LLM client using Fantasy.
package llm

import (
	"context"
	"fmt"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/bedrock"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openrouter"
	"github.com/taigrr/jety"
)

const (
	// DefaultMaxTokens is the default max output tokens.
	DefaultMaxTokens = 8192
)

func init() {
	jety.SetEnvPrefix("CRUNCH_")
	jety.SetDefault("provider", "")
	jety.SetDefault("model", "")
	jety.SetDefault("api_key", "")
}

// Provider represents a supported LLM provider.
type Provider string

const (
	ProviderBedrock    Provider = "bedrock"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderOpenRouter Provider = "openrouter"
)

// Usage represents token usage and cost information.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	Cost         float64
}

// Pricing holds model pricing info from catwalk.
type Pricing struct {
	CostPer1MIn  float64
	CostPer1MOut float64
}

// Client wraps a Fantasy agent.
type Client struct {
	agent     fantasy.Agent
	maxTokens int64
	pricing   Pricing
	provider  Provider
	model     string
}

// Options configures the LLM client.
type Options struct {
	Provider  Provider
	Model     string
	APIKey    string
	MaxTokens int
}

// Config returns the current configuration from environment variables.
func Config() (provider, model, apiKey string) {
	return jety.GetString("provider"), jety.GetString("model"), jety.GetString("api_key")
}

// NewClient creates a new LLM client using Fantasy.
func NewClient(ctx context.Context, opts *Options) (*Client, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Apply jety config as defaults
	envProvider, envModel, envAPIKey := Config()

	provider := opts.Provider
	if provider == "" && envProvider != "" {
		var err error
		provider, err = ParseProvider(envProvider)
		if err != nil {
			return nil, err
		}
	}
	if provider == "" {
		var err error
		provider, err = detectProvider()
		if err != nil {
			return nil, err
		}
	}

	model := opts.Model
	if model == "" && envModel != "" {
		model = envModel
	}
	if model == "" {
		model = defaultModelForProvider(provider)
	}

	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxTokens
	}

	apiKey := opts.APIKey
	if apiKey == "" && envAPIKey != "" {
		apiKey = envAPIKey
	}
	if apiKey == "" {
		apiKey = apiKeyFromEnv(provider)
	}

	fantasyProvider, err := createProvider(provider, apiKey)
	if err != nil {
		return nil, fmt.Errorf("creating provider: %w", err)
	}

	languageModel, err := fantasyProvider.LanguageModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}

	agent := fantasy.NewAgent(languageModel)

	// Fetch pricing from catwalk
	pricing := fetchPricing(ctx, provider, model)

	return &Client{
		agent:     agent,
		maxTokens: int64(maxTokens),
		pricing:   pricing,
		provider:  provider,
		model:     model,
	}, nil
}

const defaultCatwalkURL = "https://catwalk.charm.land"

func fetchPricing(ctx context.Context, provider Provider, model string) Pricing {
	catwalkURL := jety.GetString("CATWALK_URL")
	if catwalkURL == "" {
		catwalkURL = defaultCatwalkURL
	}
	client := catwalk.NewWithURL(catwalkURL)
	providers, err := client.GetProviders(ctx, "")
	if err != nil {
		return Pricing{}
	}

	// Map our provider to catwalk's inference provider
	// Bedrock uses Anthropic pricing since it's the same models
	var catwalkProvider catwalk.InferenceProvider
	lookupModel := model
	switch provider {
	case ProviderBedrock:
		catwalkProvider = catwalk.InferenceProviderAnthropic
		// Map bedrock model ID to anthropic model ID
		// e.g. "us.anthropic.claude-sonnet-4-20250514-v1:0" -> "claude-sonnet-4-20250514"
		lookupModel = bedrockToAnthropicModel(model)
	case ProviderAnthropic:
		catwalkProvider = catwalk.InferenceProviderAnthropic
	case ProviderOpenAI:
		catwalkProvider = catwalk.InferenceProviderOpenAI
	case ProviderOpenRouter:
		catwalkProvider = catwalk.InferenceProviderOpenRouter
	default:
		return Pricing{}
	}

	for _, p := range providers {
		if p.ID != catwalkProvider {
			continue
		}
		for _, m := range p.Models {
			if m.ID == lookupModel {
				return Pricing{
					CostPer1MIn:  m.CostPer1MIn,
					CostPer1MOut: m.CostPer1MOut,
				}
			}
		}
	}

	return Pricing{}
}

// bedrockToAnthropicModel converts a bedrock model ID to an anthropic model ID for pricing lookup.
// e.g. "us.anthropic.claude-sonnet-4-20250514-v1:0" -> "claude-sonnet-4-20250514"
func bedrockToAnthropicModel(bedrockModel string) string {
	model := bedrockModel
	// Strip region prefix (e.g., "us.")
	if idx := strings.Index(model, "anthropic."); idx != -1 {
		model = model[idx+len("anthropic."):]
	}
	// Strip version suffix (e.g., "-v1:0")
	if idx := strings.Index(model, "-v"); idx != -1 {
		model = model[:idx]
	}
	return model
}

// CalculateCost calculates the cost for the given token usage.
func (c *Client) CalculateCost(inputTokens, outputTokens int64) float64 {
	inputCost := float64(inputTokens) * c.pricing.CostPer1MIn / 1_000_000
	outputCost := float64(outputTokens) * c.pricing.CostPer1MOut / 1_000_000
	return inputCost + outputCost
}

// GetPricing returns the pricing info for the current model.
func (c *Client) GetPricing() Pricing {
	return c.pricing
}

// GetProvider returns the provider.
func (c *Client) GetProvider() Provider {
	return c.provider
}

// GetModel returns the model ID.
func (c *Client) GetModel() string {
	return c.model
}

// ErrNoCredentials is returned when no LLM provider credentials are found.
var ErrNoCredentials = fmt.Errorf("no LLM provider credentials found; set one of: CRUNCH_API_KEY, ANTHROPIC_API_KEY, OPENAI_API_KEY, OPENROUTER_API_KEY, or AWS credentials for Bedrock (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY or AWS_PROFILE)")

func detectProvider() (Provider, error) {
	if jety.GetString("api_key") != "" {
		// If CRUNCH_API_KEY is set but no provider, default to anthropic
		return ProviderAnthropic, nil
	}
	if jety.IsSet("ANTHROPIC_API_KEY") {
		return ProviderAnthropic, nil
	}
	if jety.IsSet("OPENAI_API_KEY") {
		return ProviderOpenAI, nil
	}
	if jety.IsSet("OPENROUTER_API_KEY") {
		return ProviderOpenRouter, nil
	}
	// Check for AWS credentials before defaulting to Bedrock
	if hasAWSCredentials() {
		return ProviderBedrock, nil
	}
	return "", ErrNoCredentials
}

// hasAWSCredentials checks if AWS credentials are available via environment variables or profile.
func hasAWSCredentials() bool {
	// Check for explicit credentials
	if jety.IsSet("AWS_ACCESS_KEY_ID") && jety.IsSet("AWS_SECRET_ACCESS_KEY") {
		return true
	}
	// Check for profile-based credentials
	if jety.IsSet("AWS_PROFILE") {
		return true
	}
	// Check for SSO session
	if jety.IsSet("AWS_SSO_SESSION") {
		return true
	}
	// Check for web identity (EKS/IRSA)
	if jety.IsSet("AWS_WEB_IDENTITY_TOKEN_FILE") && jety.IsSet("AWS_ROLE_ARN") {
		return true
	}
	return false
}

func defaultModelForProvider(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return "claude-sonnet-4-20250514"
	case ProviderOpenAI:
		return "gpt-4o"
	case ProviderOpenRouter:
		return "anthropic/claude-sonnet-4"
	case ProviderBedrock:
		return "us.anthropic.claude-sonnet-4-20250514-v1:0"
	default:
		return "us.anthropic.claude-sonnet-4-20250514-v1:0"
	}
}

func apiKeyFromEnv(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return jety.GetString("ANTHROPIC_API_KEY")
	case ProviderOpenAI:
		return jety.GetString("OPENAI_API_KEY")
	case ProviderOpenRouter:
		return jety.GetString("OPENROUTER_API_KEY")
	default:
		return ""
	}
}

func createProvider(p Provider, apiKey string) (fantasy.Provider, error) {
	switch p {
	case ProviderAnthropic:
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return anthropic.New(anthropic.WithAPIKey(apiKey))
	case ProviderOpenAI:
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY not set")
		}
		return openai.New(openai.WithAPIKey(apiKey))
	case ProviderOpenRouter:
		if apiKey == "" {
			return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
		}
		return openrouter.New(openrouter.WithAPIKey(apiKey))
	case ProviderBedrock:
		return bedrock.New()
	default:
		return nil, fmt.Errorf("unknown provider: %s", p)
	}
}

// Invoke sends a prompt to the model and returns the response text.
func (c *Client) Invoke(ctx context.Context, prompt string) (string, error) {
	maxTokens := c.maxTokens
	result, err := c.agent.Generate(ctx, fantasy.AgentCall{
		Prompt:          prompt,
		MaxOutputTokens: &maxTokens,
	})
	if err != nil {
		return "", err
	}

	return result.Response.Content.Text(), nil
}

// StreamCall represents parameters for a streaming call.
type StreamCall struct {
	Prompt         string
	OnTextDelta    func(text string) error
	OnStreamFinish func(usage Usage) error
}

// InvokeStream sends a prompt and streams the response.
func (c *Client) InvokeStream(ctx context.Context, call StreamCall) (string, error) {
	maxTokens := c.maxTokens
	result, err := c.agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt:          call.Prompt,
		MaxOutputTokens: &maxTokens,
		OnTextDelta: func(id, text string) error {
			if call.OnTextDelta != nil {
				return call.OnTextDelta(text)
			}
			return nil
		},
		OnStreamFinish: func(usage fantasy.Usage, finishReason fantasy.FinishReason, providerMetadata fantasy.ProviderMetadata) error {
			if call.OnStreamFinish != nil {
				cost := c.CalculateCost(usage.InputTokens, usage.OutputTokens)
				return call.OnStreamFinish(Usage{
					InputTokens:  usage.InputTokens,
					OutputTokens: usage.OutputTokens,
					TotalTokens:  usage.TotalTokens,
					Cost:         cost,
				})
			}
			return nil
		},
	})
	if err != nil {
		return "", err
	}

	return result.Response.Content.Text(), nil
}

// SupportedProviders returns a list of supported provider names.
func SupportedProviders() []string {
	return []string{
		string(ProviderBedrock),
		string(ProviderAnthropic),
		string(ProviderOpenAI),
		string(ProviderOpenRouter),
	}
}

// ParseProvider parses a provider string.
func ParseProvider(s string) (Provider, error) {
	switch strings.ToLower(s) {
	case "bedrock", "aws":
		return ProviderBedrock, nil
	case "anthropic", "claude":
		return ProviderAnthropic, nil
	case "openai", "gpt":
		return ProviderOpenAI, nil
	case "openrouter":
		return ProviderOpenRouter, nil
	default:
		return "", fmt.Errorf("unknown provider: %s (supported: %s)", s, strings.Join(SupportedProviders(), ", "))
	}
}
