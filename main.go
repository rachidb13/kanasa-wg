package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

/* ========= GLOBAL ========= */

var localServerKey string

/* ========= DATA STRUCTS ========= */

type PeerAddRequest struct {
	ServerKey string `json:"server_key"`
	PublicKey string `json:"public_key"`
	IP        string `json:"ip"`
}

type ApiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

/* ========= HANDLERS ========= */

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func peerAddHandler(w http.ResponseWriter, r *http.Request) {
	var req PeerAddRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// SAFETY: only allow execution on this VPS
	if req.ServerKey != localServerKey {
		http.Error(w, "execution not allowed for this server", http.StatusForbidden)
		return
	}

	log.Printf("Adding WG peer: %s %s", req.PublicKey, req.IP)

	// 1. wg set
	cmd := exec.Command(
		"wg", "set", "wg0",
		"peer", req.PublicKey,
		"allowed-ips", req.IP,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("wg set failed: %s", string(out))
		http.Error(w, "wg set failed", http.StatusInternalServerError)
		return
	}

	// 2. verify
	verifyCmd := exec.Command("wg", "show", "wg0", "allowed-ips")
	verifyOut, err := verifyCmd.CombinedOutput()
	if err != nil {
		log.Printf("wg verify failed: %s", string(verifyOut))
		http.Error(w, "wg verify failed", http.StatusInternalServerError)
		return
	}

	if !strings.Contains(string(verifyOut), req.PublicKey) {
		log.Printf("peer not found after wg set")
		http.Error(w, "peer verification failed", http.StatusInternalServerError)
		return
	}

	resp := ApiResponse{
		Success: true,
		Message: "peer added successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func peerRemoveHandler(w http.ResponseWriter, r *http.Request) {
	var req PeerAddRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// SAFETY: only allow execution on this VPS
	if req.ServerKey != localServerKey {
		http.Error(w, "execution not allowed for this server", http.StatusForbidden)
		return
	}

	log.Printf("Removing WG peer: %s", req.PublicKey)

	cmd := exec.Command(
		"wg", "set", "wg0",
		"peer", strings.TrimSpace(req.PublicKey),
		"remove",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("wg remove failed: %s", string(out))
		http.Error(w, "wg remove failed", http.StatusInternalServerError)
		return
	}

	resp := ApiResponse{
		Success: true,
		Message: "peer removed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

/* ========= MAIN ========= */

func main() {
	localServerKey = os.Getenv("KANASA_SERVER_KEY")
	if localServerKey == "" {
		log.Fatal("KANASA_SERVER_KEY is not set")
	}

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/peer/add", peerAddHandler)
	http.HandleFunc("/peer/remove", peerRemoveHandler)

	log.Printf(
		"Kanasa WG service for server_key=%s listening on 127.0.0.1:9000",
		localServerKey,
	)

	log.Fatal(http.ListenAndServe("127.0.0.1:9000", nil))
}
