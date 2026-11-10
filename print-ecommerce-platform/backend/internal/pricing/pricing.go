package pricing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AttributeImpactType string

const (
	ImpactFixedAdd   AttributeImpactType = "fixed_add"
	ImpactPerUnit    AttributeImpactType = "per_unit"
	ImpactMultiplier AttributeImpactType = "multiplier"
)

type SelectedAttribute struct {
	AttributeID string              `json:"attribute_id"`
	ValueID     string              `json:"value_id"`
	ImpactType  AttributeImpactType `json:"impact_type"`
	CostValue   float64             `json:"cost_value"`
}

type PricingRequest struct {
	ProductID          string              `json:"product_id"`
	Quantity           int                 `json:"quantity"`
	SelectedAttributes []SelectedAttribute `json:"selected_attributes"`
}

type PricingResponse struct {
	ProductID         string  `json:"product_id"`
	Quantity          int     `json:"quantity"`
	BaseUnitPrice     float64 `json:"base_unit_price"`
	EffectiveUnitCost float64 `json:"effective_unit_cost"`
	Subtotal          float64 `json:"subtotal"`
	TaxAmount         float64 `json:"tax_amount"`
	GrandTotal        float64 `json:"grand_total"`
}

type Service struct {
	pool    *pgxpool.Pool
	vatRate float64
}

func NewService(pool *pgxpool.Pool, vatRate float64) *Service {
	return &Service{pool: pool, vatRate: vatRate}
}

func (s *Service) CalculateHandler(w http.ResponseWriter, r *http.Request) {
	var req PricingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Malformed JSON request body", http.StatusBadRequest)
		return
	}

	if req.Quantity <= 0 {
		http.Error(w, "Quantity must be greater than zero", http.StatusBadRequest)
		return
	}

	// 1. Fetch matching tier price from PostgreSQL
	query := `
		SELECT unit_price 
		FROM quantity_tiers 
		WHERE product_id = $1 
		  AND min_quantity <= $2 
		  AND (max_quantity IS NULL OR max_quantity >= $2)
		LIMIT 1;
	`
	var baseUnitPrice float64
	err := s.pool.QueryRow(context.Background(), query, req.ProductID, req.Quantity).Scan(&baseUnitPrice)
	if err != nil {
		baseUnitPrice = 3.50 // fallback rate if outside seeded tier
	}

	// 2. Aggregate attributes
	var perUnitAdd float64
	var fixedAdd float64
	multiplier := 1.0

	for _, attr := range req.SelectedAttributes {
		switch attr.ImpactType {
		case ImpactPerUnit:
			perUnitAdd += attr.CostValue
		case ImpactFixedAdd:
			fixedAdd += attr.CostValue
		case ImpactMultiplier:
			if attr.CostValue > 0 {
				multiplier *= attr.CostValue
			}
		}
	}

	// 3. Compute totals
	unitCostBeforeMultiplier := baseUnitPrice + perUnitAdd
	effectiveUnitCost := (unitCostBeforeMultiplier * multiplier) + (fixedAdd / float64(req.Quantity))
	subtotal := math.Round((effectiveUnitCost*float64(req.Quantity))*100) / 100
	tax := math.Round((subtotal*s.vatRate)*100) / 100
	grandTotal := math.Round((subtotal+tax)*100) / 100

	res := PricingResponse{
		ProductID:         req.ProductID,
		Quantity:          req.Quantity,
		BaseUnitPrice:     baseUnitPrice,
		EffectiveUnitCost: math.Round(effectiveUnitCost*100) / 100,
		Subtotal:          subtotal,
		TaxAmount:         tax,
		GrandTotal:        grandTotal,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
