package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
    "print-ecommerce/backend/internal/pricing"
)

func main() {
    dbURL := "postgres://postgres:rootpassword@localhost:5432/print_ecommerce?sslmode=disable"
    dbPool, err := pgxpool.New(context.Background(), dbURL)
    if err != nil {
        log.Fatalf("Unable to connect to database: %v\n", err)
    }
    defer dbPool.Close()

    if err := dbPool.Ping(context.Background()); err != nil {
        log.Fatalf("Database ping failed: %v\n", err)
    }

    fmt.Println("PostgreSQL connection established successfully.")

    http.HandleFunc("/api/v1/pricing/calculate", pricing.NewHandler(dbPool).Calculate)

    http.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok","database":"connected"}`))
    })

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    fmt.Printf("Server listening on port %s\n", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("Server failed: %v\n", err)
    }
}
