package gcp

import (
	"context"
	"gpu-orchestrator/core/models"
)

// Client is the GCP provider client
type Client struct {
	projectID string
	regions   []string
}

// NewClient creates a new GCP client
func NewClient(ctx context.Context, projectID string, regions []string) (*Client, error) {
	// TODO: Initialize GCP client
	return &Client{
		projectID: projectID,
		regions:   regions,
	}, nil
}

// FetchOnDemandPricing fetches on-demand pricing from GCP
func (c *Client) FetchOnDemandPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement GCP Pricing API calls
	return []models.GPUInstance{}, nil
}

// FetchPreemptiblePricing fetches preemptible pricing from GCP
func (c *Client) FetchPreemptiblePricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement GCP Preemptible pricing
	return []models.GPUInstance{}, nil
}
