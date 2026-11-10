package pricing

import (
    "encoding/json"
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
    db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
    return &Handler{db: db}
}

type CalculatePriceRequest struct {
    ProductID          string              `json:"product_id"`
    Quantity           int                 `json:"quantity"`
    SelectedAttributes []SelectedAttribute `json:"selected_attributes"`
}

type CalculatePriceResponse struct {
    BaseUnitPrice float64 `json:"base_unit_price"`
    AttributesCost float64 `json:"attributes_cost"`
    Subtotal      float64 `json:"subtotal"`
    TaxAmount     float64 `json:"tax_amount"`
    GrandTotal    float64 `json:"grand_total"`
}

func (h *Handler) Calculate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req CalculatePriceRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
        return
    }

    // 1. Fetch base unit price from quantity_tiers
    var unitPrice float64
    query := `
        SELECT unit_price 
        FROM quantity_tiers 
        WHERE product_id = $1 
          AND min_quantity <= $2 
          AND (max_quantity IS NULL OR max_quantity >= $2)
        LIMIT 1;
    `
    err := h.db.QueryRow(r.Context(), query, req.ProductID, req.Quantity).Scan(&unitPrice)
    if err != nil {
        http.Error(w, "Quantity tier or product not found: "+err.Error(), http.StatusNotFound)
        return
    }

    // 2. Compute dynamic attribute adjustments
    var attributesCost float64
    for _, attr := range req.SelectedAttributes {
        switch attr.ImpactType {
        case ImpactPerUnit:
            attributesCost += attr.CostValue * float64(req.Quantity)
        case ImpactFixedAdd:
            attributesCost += attr.CostValue
        }
    }

    subtotal := (unitPrice * float64(req.Quantity)) + attributesCost
    tax := subtotal * 0.15 // 15% standard VAT
    grandTotal := subtotal + tax

    res := CalculatePriceResponse{
        BaseUnitPrice:  unitPrice,
        AttributesCost: attributesCost,
        Subtotal:       subtotal,
        TaxAmount:      tax,
        GrandTotal:     grandTotal,
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(res)
}
