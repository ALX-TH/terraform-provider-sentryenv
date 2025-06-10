package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type SentryEnvEnvironmentResource struct{}

func NewSentryEnvEnvironmentResource() resource.Resource {
	return &SentryEnvEnvironmentResource{}
}

// Resource model for state
type EnvironmentModel struct {
	ID          types.String `tfsdk:"id"`
	AuthToken   types.String `tfsdk:"auth_token"`
	Slug        types.String `tfsdk:"slug"`
	Dsn         types.String `tfsdk:"dsn"`
	ProjectName types.String `tfsdk:"project_name"`
	Envs        types.String `tfsdk:"envs"`
	Hash        types.String `tfsdk:"hash"`
}

func deleteIssueByEventID(ctx context.Context, eventID, organization, authToken, sentryHost, protocol string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	queryURL := fmt.Sprintf("%s://%s/api/0/organizations/%s/issues/?query=id:%s", protocol, sentryHost, organization, eventID)

	var issueID string
	for i := 0; i < 10; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("getting issue: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var issues []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
				return fmt.Errorf("decoding issue list: %w", err)
			}
			if len(issues) > 0 {
				if id, ok := issues[0]["id"].(string); ok {
					issueID = id
					break
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	if issueID == "" {
		return fmt.Errorf("issue not found for event ID: %s", eventID)
	}

	deleteURL := fmt.Sprintf("%s://%s/api/0/organizations/%s/issues/%s/", protocol, sentryHost, organization, issueID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response deleting issue: %s", string(body))
	}

	return nil
}

func (r *SentryEnvEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "sentryenv_environment"
}

func (r *SentryEnvEnvironmentResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"auth_token": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
			"slug": schema.StringAttribute{
				Required: true,
			},
			"dsn": schema.StringAttribute{
				Required: true,
			},
			"project_name": schema.StringAttribute{
				Required: true,
			},
			"envs": schema.StringAttribute{
				Required: true,
			},
			"hash": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *SentryEnvEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentModel
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse DSN
	dsn := data.Dsn.ValueString()
	parts := strings.Split(dsn, "@")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid DSN", "DSN must be in format https://<key>@<host>/<project_id>")
		return
	}

	protocolAndKey := parts[0]
	hostAndProject := parts[1]

	protocol := strings.Split(protocolAndKey, ":")[0]
	sentryKey := strings.TrimPrefix(protocolAndKey, protocol+"://")
	sentryKey = strings.TrimSuffix(sentryKey, "/")

	hostParts := strings.Split(hostAndProject, "/")
	if len(hostParts) < 2 {
		resp.Diagnostics.AddError("Invalid DSN", "DSN missing host or project id")
		return
	}
	sentryHost := hostParts[0]
	projectID := hostParts[len(hostParts)-1]

	if sentryKey == "" || sentryHost == "" || projectID == "" {
		resp.Diagnostics.AddError("Invalid DSN", "Could not parse key, host or project_id from DSN")
		return
	}

	envsCSV := data.Envs.ValueString()
	envList := strings.Split(envsCSV, ",")

	eventIDs := []string{}
	timestamp := time.Now().UTC().Format(time.RFC3339)

	authHeader := fmt.Sprintf("Sentry sentry_version=7,sentry_client=sentry.go.custom/0.1.0,sentry_timestamp=%s,sentry_key=%s", timestamp, sentryKey)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, env := range envList {
		env = strings.TrimSpace(env)
		eventID := strings.ReplaceAll(uuid.New().String(), "-", "")
		eventIDs = append(eventIDs, eventID)

		payload := fmt.Sprintf(`{
            "event_id": "%s",
            "message": "Terraform automated %s project %s environment creator",
            "timestamp": "%s",
            "level": "info",
            "platform": "other",
            "logger": "terraform",
            "environment": "%s",
            "sdk": {
                "name": "sentry.go.custom",
                "version": "0.1.0"
            }
        }`, eventID, data.ProjectName.ValueString(), env, timestamp, env)

		reqURL := fmt.Sprintf("%s://%s/api/%s/store/", protocol, sentryHost, projectID)
		httpReq, err := http.NewRequest("POST", reqURL, strings.NewReader(payload))
		if err != nil {
			resp.Diagnostics.AddError("Request creation failed", err.Error())
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Sentry-Auth", authHeader)

		httpResp, err := client.Do(httpReq)
		if err != nil {
			resp.Diagnostics.AddError("Request failed", err.Error())
			return
		}
		if httpResp.StatusCode != 200 {
			resp.Diagnostics.AddError("Sentry API error", fmt.Sprintf("Status: %d", httpResp.StatusCode))
			return
		}
		httpResp.Body.Close()

		err = deleteIssueByEventID(ctx, eventID, data.Slug.ValueString(), data.AuthToken.ValueString(), sentryHost, protocol)
		if err != nil {
			resp.Diagnostics.AddError("Failed to delete issue", err.Error())
		}
	}

	resourceID := strings.Join(eventIDs, "-")
	h := md5.New()
	h.Write([]byte(resourceID))
	hash := hex.EncodeToString(h.Sum(nil))

	data.ID = types.StringValue(data.ProjectName.ValueString())
	data.Hash = types.StringValue(hash)

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *SentryEnvEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// No remote state to read, so just keep state as is
}

func (r *SentryEnvEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Just call Create to refresh
	r.Create(ctx, resource.CreateRequest{Plan: req.Plan}, &resource.CreateResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	})
}

func (r *SentryEnvEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: since events are ephemeral, just remove from state
}
