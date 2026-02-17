package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Response format expected by CloudVault's PrometheusClient
type PromResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [timestamp, "string_value"]
		} `json:"result"`
	} `json:"data"`
}

func main() {
	http.HandleFunc("/api/v1/query", handleQuery)
	port := ":9090"
	fmt.Printf("🔥 Mock Prometheus running on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	fmt.Printf("📥 Query: %s\n", query)

	// Default response structure
	resp := PromResponse{
		Status: "success",
	}
	resp.Data.ResultType = "vector"

	// Generate mock data based on query content
	// We match the PVCs seen in the user's cluster output

	value := "0"

	// Simulate "postgres-data" (Active, 50% full)
	if strings.Contains(query, "postgres-data") {
		if strings.Contains(query, "used_bytes") {
			value = "53687091200" // 50GB
		}
	} else if strings.Contains(query, "postgres-backup") {
		// Simulate "postgres-backup" (Low usage, 10GB used of 200GB -> 5% -> Oversized)
		if strings.Contains(query, "used_bytes") {
			value = "10737418240" // 10GB
		}
	} else if strings.Contains(query, "old-logs-archive") {
		// Simulate "old-logs-archive" (Zombie? No, lets make it empty but unused)
		if strings.Contains(query, "used_bytes") {
			value = "1024" // 1KB (Empty)
		}
		// Activity query would return 0/empty, triggering Zombie logic in CloudVault if we had implemented the 2nd query fully.
		// NOTE: Current CloudVault implementation mainly uses UsedBytes.
	} else if strings.Contains(query, "test-db") {
		// Simulate "test-db" (Oversized, 20GB size, 100MB usage)
		if strings.Contains(query, "used_bytes") {
			value = "104857600" // 100MB
		}
	}

	// Construct result
	// timestamp is current time, value is string
	now := float64(time.Now().Unix())
	result := struct {
		Metric map[string]string `json:"metric"`
		Value  []interface{}     `json:"value"`
	}{
		Metric: map[string]string{},
		Value:  []interface{}{now, value},
	}

	resp.Data.Result = append(resp.Data.Result, result)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Printf("Error encoding response: %v\n", err)
	}
}
