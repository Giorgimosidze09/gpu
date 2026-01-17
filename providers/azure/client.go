package azure

import (
	"context"
	"gpu-orchestrator/core/models"
)

// Client is the Azure provider client
type Client struct {
	subscriptionID string
	regions        []string
}

// NewClient creates a new Azure client
func NewClient(ctx context.Context, subscriptionID string, regions []string) (*Client, error) {
	// TODO: Initialize Azure client
	return &Client{
		subscriptionID: subscriptionID,
		regions:        regions,
	}, nil
}

// FetchOnDemandPricing fetches on-demand pricing from Azure
func (c *Client) FetchOnDemandPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement Azure Pricing API calls
	return []models.GPUInstance{}, nil
}

// FetchSpotPricing fetches spot pricing from Azure
func (c *Client) FetchSpotPricing(ctx context.Context) ([]models.GPUInstance, error) {
	// TODO: Implement Azure Spot pricing
	return []models.GPUInstance{}, nil
}
