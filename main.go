package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func getRoot(res http.ResponseWriter, req *http.Request) {
	_, err := fmt.Fprint(res, "This is a root page!")
	if err != nil {
		return
	}
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Name    string `json:"name"`
}

func getHealth(res http.ResponseWriter, req *http.Request) {
	response := HealthResponse{
		Status:  "healthy",
		Version: "v1",
		Name:    "Workshop 01 - Cat Gallery",
	}

	// Marshalling
	jsonResp, err := json.Marshal(response)
	if err != nil {
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	_, err = res.Write(jsonResp)
	if err != nil {
		return
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", getRoot)
	mux.HandleFunc("GET /healthcheck", getHealth)

	log.Print("Server listening on port :3000")
	err := http.ListenAndServe(":3000", mux)
	if err != nil {
		panic(err)
	}
}
