package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func NewProvider() provider.Provider {
	return &SentryEnvProvider{}
}

type SentryEnvProvider struct{}

func (p *SentryEnvProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "sentryenv"
}

func (p *SentryEnvProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{}
}

func (p *SentryEnvProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
	// No-op for now
}

func (p *SentryEnvProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSentryEnvEnvironmentResource,
	}
}

func (p *SentryEnvProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
