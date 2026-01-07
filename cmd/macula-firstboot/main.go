package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/skip2/go-qrcode"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

const (
	configuredFlag = "/var/lib/macula/.configured"
	credentialsDir = "/var/lib/macula/credentials"
	defaultPort    = 80
	portalURL      = "https://macula.io"
)

var (
	port       int
	forceRun   bool
	portalHost string
)

func init() {
	flag.IntVar(&port, "port", defaultPort, "HTTP server port")
	flag.BoolVar(&forceRun, "force", false, "Force run even if already configured")
	flag.StringVar(&portalHost, "portal", portalURL, "Portal URL for pairing")
}

func main() {
	flag.Parse()

	// Check if already configured
	if !forceRun && isConfigured() {
		log.Println("System already configured. Use -force to run anyway.")
		os.Exit(0)
	}

	// Generate pairing code
	pairingCode := generatePairingCode()
	hostname := getHostname()
	localURL := fmt.Sprintf("http://%s.local", hostname)
	if port != 80 {
		localURL = fmt.Sprintf("http://%s.local:%d", hostname, port)
	}

	// Print banner to console
	printBanner(hostname, pairingCode, localURL)

	// Start HTTP server
	server := NewFirstbootServer(pairingCode, hostname, localURL)
	addr := fmt.Sprintf(":%d", port)

	log.Printf("Starting firstboot server on %s", addr)
	log.Printf("Visit %s to complete setup", localURL)

	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// FirstbootServer handles the pairing flow
type FirstbootServer struct {
	pairingCode string
	hostname    string
	localURL    string
	mux         *http.ServeMux
}

func NewFirstbootServer(pairingCode, hostname, localURL string) *FirstbootServer {
	s := &FirstbootServer{
		pairingCode: pairingCode,
		hostname:    hostname,
		localURL:    localURL,
		mux:         http.NewServeMux(),
	}

	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/pair", s.handlePair)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/qr.png", s.handleQRCode)
	s.mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	return s
}

func (s *FirstbootServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *FirstbootServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
		return
	}

	data := map[string]interface{}{
		"Hostname":    s.hostname,
		"PairingCode": s.pairingCode,
		"LocalURL":    s.localURL,
		"PortalURL":   portalHost,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execute error: %v", err)
	}
}

func (s *FirstbootServer) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PortalCode string `json:"portal_code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	if req.PortalCode == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Portal code required"})
		return
	}

	// Exchange codes with Portal
	result, err := exchangeCodesWithPortal(req.PortalCode, s.pairingCode)
	if err != nil {
		log.Printf("Pairing failed: %v", err)
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Store credentials
	if err := storeCredentials(result); err != nil {
		log.Printf("Failed to store credentials: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to store credentials"})
		return
	}

	// Mark as configured
	if err := markConfigured(); err != nil {
		log.Printf("Failed to mark configured: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "Pairing successful! Rebooting in 5 seconds...",
		"org_name": result.OrgIdentity,
	})

	// Schedule reboot
	go func() {
		time.Sleep(5 * time.Second)
		log.Println("Rebooting system...")
		exec.Command("reboot").Run()
	}()
}

func (s *FirstbootServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"configured":   isConfigured(),
		"hostname":     s.hostname,
		"pairing_code": s.pairingCode,
	})
}

func (s *FirstbootServer) handleQRCode(w http.ResponseWriter, r *http.Request) {
	// Generate QR code for the local URL
	qr, err := qrcode.Encode(s.localURL, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(qr)
}

// Portal API types
type PairingResult struct {
	UserName     string `json:"user_name"`
	OrgIdentity  string `json:"org_identity"`
	RefreshToken string `json:"refresh_token"`
}

func exchangeCodesWithPortal(portalCode, nodeCode string) (*PairingResult, error) {
	payload := map[string]string{
		"pairing_code": portalCode,
		"node_code":    nodeCode,
		"hostname":     getHostname(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/console/pair", portalHost)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to contact Portal: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("pairing failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("pairing failed with status %d", resp.StatusCode)
	}

	var result PairingResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func storeCredentials(result *PairingResult) error {
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return err
	}

	// Store as JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	credPath := filepath.Join(credentialsDir, "portal.json")
	return os.WriteFile(credPath, data, 0600)
}

func isConfigured() bool {
	_, err := os.Stat(configuredFlag)
	return err == nil
}

func markConfigured() error {
	if err := os.MkdirAll(filepath.Dir(configuredFlag), 0755); err != nil {
		return err
	}
	return os.WriteFile(configuredFlag, []byte(time.Now().Format(time.RFC3339)), 0644)
}

func generatePairingCode() string {
	// Generate a 6-character alphanumeric code (e.g., "ABC-123")
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Avoid ambiguous chars
	b := make([]byte, 6)
	rand.Read(b)

	code := make([]byte, 6)
	for i := range code {
		code[i] = charset[int(b[i])%len(charset)]
	}

	return fmt.Sprintf("%s-%s", string(code[:3]), string(code[3:]))
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "macula-node"
	}
	return hostname
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}

func printBanner(hostname, pairingCode, localURL string) {
	banner := `
  __  __                 _        ___  ____
 |  \/  | __ _  ___ _   _| | __ _ / _ \/ ___|
 | |\/| |/ _` + "`" + ` |/ __| | | | |/ _` + "`" + ` | | | \___ \
 | |  | | (_| | (__| |_| | | (_| | |_| |___) |
 |_|  |_|\__,_|\___|\__,_|_|\__,_|\___/|____/

 ╔════════════════════════════════════════════════════════╗
 ║            FIRST-TIME SETUP REQUIRED                   ║
 ╠════════════════════════════════════════════════════════╣
 ║                                                        ║
 ║  Visit: %-42s  ║
 ║                                                        ║
 ║  Or scan the QR code on screen                        ║
 ║                                                        ║
 ║  Pairing Code: %-6s                                  ║
 ║                                                        ║
 ╚════════════════════════════════════════════════════════╝
`
	fmt.Printf(banner, localURL, pairingCode)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Placeholder for QR code display on console (framebuffer)
func displayQROnConsole(url string) error {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return err
	}

	// Print ASCII QR to console
	fmt.Println(qr.ToSmallString(false))
	return nil
}
