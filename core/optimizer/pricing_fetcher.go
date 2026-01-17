package optimizer

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"gpu-orchestrator/core/models"
	"gpu-orchestrator/providers/aws"
	"gpu-orchestrator/providers/azure"
	"gpu-orchestrator/providers/gcp"
)

// PricingFetcher fetches and caches GPU pricing from all providers
type PricingFetcher struct {
	awsClient   *aws.Client
	gcpClient   *gcp.Client
	azureClient *azure.Client
	db          *sql.DB
	cacheTTL    time.Duration
	mu          sync.RWMutex
}

// NewPricingFetcher creates a new pricing fetcher
func NewPricingFetcher(
	awsClient *aws.Client,
	gcpClient *gcp.Client,
	azureClient *azure.Client,
	db *sql.DB,
) *PricingFetcher {
	if db == nil {
		// Return nil if no database (for testing)
		return nil
	}
	return &PricingFetcher{
		awsClient:   awsClient,
		gcpClient:   gcpClient,
		azureClient: azureClient,
		db:          db,
		cacheTTL:    15 * time.Minute, // Refresh every 15 minutes
	}
}

// StartRefreshWorker starts a background worker to refresh pricing from provider APIs
func (pf *PricingFetcher) StartRefreshWorker(ctx context.Context) {
	ticker := time.NewTicker(pf.cacheTTL)
	defer ticker.Stop()

	// Initial refresh
	pf.refreshAllPricing(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pf.refreshAllPricing(ctx)
		}
	}
}

func (pf *PricingFetcher) refreshAllPricing(ctx context.Context) {
	// Fetch on-demand pricing from provider APIs (stable)
	if pf.awsClient != nil {
		awsPricing, err := pf.awsClient.FetchOnDemandPricing(ctx)
		if err == nil {
			pf.storePricing(awsPricing)
		}

		// Fetch spot pricing (probabilistic - uses EC2 Spot Price History)
		awsSpotPricing, err := pf.awsClient.FetchSpotPricing(ctx)
		if err == nil {
			pf.storeSpotPricing(awsSpotPricing)
		}
	}

	// Fetch GCP on-demand + preemptible (similar approach)
	if pf.gcpClient != nil {
		gcpPricing, err := pf.gcpClient.FetchOnDemandPricing(ctx)
		if err == nil {
			pf.storePricing(gcpPricing)
		}

		gcpPreemptiblePricing, err := pf.gcpClient.FetchPreemptiblePricing(ctx)
		if err == nil {
			pf.storePreemptiblePricing(gcpPreemptiblePricing)
		}
	}

	// Fetch Azure on-demand + spot (similar approach)
	if pf.azureClient != nil {
		azurePricing, err := pf.azureClient.FetchOnDemandPricing(ctx)
		if err == nil {
			pf.storePricing(azurePricing)
		}

		azureSpotPricing, err := pf.azureClient.FetchSpotPricing(ctx)
		if err == nil {
			pf.storeSpotPricing(azureSpotPricing)
		}
	}
}

// storePricing stores pricing data in the database
func (pf *PricingFetcher) storePricing(instances []models.GPUInstance) {
	for _, instance := range instances {
		query := `
			INSERT INTO gpu_pricing (
				provider, region, instance_type, gpu_type, gpus_per_instance,
				memory_per_gpu_gb, interconnect, on_demand_price_per_hour, last_updated
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (provider, region, instance_type)
			DO UPDATE SET
				on_demand_price_per_hour = EXCLUDED.on_demand_price_per_hour,
				last_updated = NOW()
		`

		_, err := pf.db.Exec(query,
			instance.Provider,
			instance.Region,
			instance.InstanceType,
			instance.GPUType,
			instance.GPUsPerInstance,
			instance.MemoryPerGPU,
			instance.InterconnectTier,
			instance.PricePerHour,
		)
		if err != nil {
			// Log error but continue
			continue
		}
	}
}

// storeSpotPricing stores spot pricing data in the database
func (pf *PricingFetcher) storeSpotPricing(instances []models.GPUInstance) {
	for _, instance := range instances {
		query := `
			INSERT INTO gpu_pricing (
				provider, region, instance_type, gpu_type, gpus_per_instance,
				memory_per_gpu_gb, interconnect, on_demand_price_per_hour,
				spot_price_per_hour, spot_availability, last_updated
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
			ON CONFLICT (provider, region, instance_type)
			DO UPDATE SET
				spot_price_per_hour = EXCLUDED.spot_price_per_hour,
				spot_availability = EXCLUDED.spot_availability,
				last_updated = NOW()
		`

		_, err := pf.db.Exec(query,
			instance.Provider,
			instance.Region,
			instance.InstanceType,
			instance.GPUType,
			instance.GPUsPerInstance,
			instance.MemoryPerGPU,
			instance.InterconnectTier,
			instance.PricePerHour, // Keep on-demand price
			instance.SpotPrice,
			instance.Availability,
		)
		if err != nil {
			// Log error but continue
			continue
		}
	}
}

// storePreemptiblePricing stores preemptible pricing data in the database (GCP)
func (pf *PricingFetcher) storePreemptiblePricing(instances []models.GPUInstance) {
	// GCP preemptible is similar to spot pricing
	pf.storeSpotPricing(instances)
}

// FetchAllPricing fetches real-time pricing from all providers
func (pf *PricingFetcher) FetchAllPricing(ctx context.Context) (map[models.Provider][]models.GPUInstance, error) {
	// Get all instances from database (refreshed by background worker)
	return pf.GetAllInstances(ctx)
}

// GetAllInstances gets all instances from database
func (pf *PricingFetcher) GetAllInstances(ctx context.Context) (map[models.Provider][]models.GPUInstance, error) {
	query := `
        SELECT provider, instance_type, region, gpu_type, gpus_per_instance,
               memory_per_gpu_gb, on_demand_price_per_hour, spot_price_per_hour, 
               spot_availability, interconnect, last_updated
        FROM gpu_pricing
        WHERE last_updated > NOW() - INTERVAL '1 hour'
    `

	rows, err := pf.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[models.Provider][]models.GPUInstance)

	for rows.Next() {
		var instance models.GPUInstance
		var spotPrice sql.NullFloat64
		var spotAvailability sql.NullFloat64

		err := rows.Scan(
			&instance.Provider,
			&instance.InstanceType,
			&instance.Region,
			&instance.GPUType,
			&instance.GPUsPerInstance,
			&instance.MemoryPerGPU,
			&instance.PricePerHour,
			&spotPrice,
			&spotAvailability,
			&instance.InterconnectTier,
			&instance.LastUpdated,
		)
		if err != nil {
			continue
		}

		if spotPrice.Valid {
			instance.SpotPrice = spotPrice.Float64
		}
		if spotAvailability.Valid {
			instance.Availability = spotAvailability.Float64
		}

		results[instance.Provider] = append(results[instance.Provider], instance)
	}

	return results, nil
}

// GetPrice gets price from database (refreshed by background worker)
func (pf *PricingFetcher) GetPrice(provider models.Provider, instanceType string, region string, spot bool) (float64, error) {
	var price float64
	var spotPrice sql.NullFloat64
	var lastUpdated time.Time

	query := `
        SELECT on_demand_price_per_hour, spot_price_per_hour, last_updated 
        FROM gpu_pricing 
        WHERE provider = $1 AND instance_type = $2 AND region = $3
        ORDER BY last_updated DESC 
        LIMIT 1
    `

	err := pf.db.QueryRow(query, provider, instanceType, region).Scan(
		&price, &spotPrice, &lastUpdated,
	)
	if err == sql.ErrNoRows {
		// No pricing data, fetch fresh
		return pf.fetchFreshPrice(context.Background(), provider, instanceType, region, spot)
	}
	if err != nil {
		return 0, err
	}

	// Use spot price if requested and available
	if spot && spotPrice.Valid && spotPrice.Float64 > 0 {
		price = spotPrice.Float64
	}

	// If pricing is stale (> 1 hour), fetch fresh in background
	if time.Since(lastUpdated) > 1*time.Hour {
		go pf.refreshPricingForInstance(context.Background(), provider, instanceType, region)
	}

	return price, nil
}

func (pf *PricingFetcher) fetchFreshPrice(_ context.Context, _ models.Provider, _ string, _ string, _ bool) (float64, error) {
	// TODO: Implement fresh price fetching from provider APIs
	return 0, nil
}

func (pf *PricingFetcher) refreshPricingForInstance(_ context.Context, _ models.Provider, _ string, _ string) {
	// TODO: Implement per-instance refresh
}
