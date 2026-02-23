package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type flyCertResponse struct {
	Hostname    string `json:"hostname"`
	Status      string `json:"status"`
	Configured  bool   `json:"configured"`
	DNSProvider string `json:"dns_provider"`
	Certificates []struct {
		Source    string `json:"source"`
		Status   string `json:"status"`
		ExpiresAt string `json:"expires_at"`
		Issued   []struct {
			Type               string `json:"type"`
			ExpiresAt          string `json:"expires_at"`
			CertificateAuthority string `json:"certificate_authority"`
		} `json:"issued"`
	} `json:"certificates"`
	DNSRequirements struct {
		A    []string `json:"a"`
		AAAA []string `json:"aaaa"`
	} `json:"dns_requirements"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "1234"
	}

	appName := os.Getenv("FLY_APP_NAME")
	if appName == "" {
		appName = "from-root-to-www"
	}
	apiToken := os.Getenv("FLY_API_TOKEN")
	apiBase := "https://api.machines.dev/v1/apps/" + appName

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	http.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/status/")
		domain = strings.TrimSpace(domain)
		if domain == "" {
			http.Error(w, "Usage: /status/<domain>", http.StatusBadRequest)
			return
		}
		for _, c := range domain {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
				http.Error(w, "Invalid domain", http.StatusBadRequest)
				return
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "=== %s ===\n\n", domain)

		if apiToken == "" {
			fmt.Fprintf(w, "ERROR: FLY_API_TOKEN not set\n")
			return
		}

		// Try to get existing certificate
		certInfo, created, err := getOrCreateCert(apiBase, apiToken, appName, domain)
		if err != nil {
			fmt.Fprintf(w, "ERROR: %v\n", err)
			return
		}

		if created {
			fmt.Fprintf(w, "Certificate: CREATED (new)\n")
		} else {
			fmt.Fprintf(w, "Certificate: %s\n", certInfo.Status)
		}

		if certInfo.DNSProvider != "" {
			fmt.Fprintf(w, "DNS Provider: %s\n", certInfo.DNSProvider)
		}
		fmt.Fprintln(w)

		// Show DNS status
		fmt.Fprintf(w, "DNS:\n")
		aRecords, err := net.LookupHost(domain)
		if err != nil {
			fmt.Fprintf(w, "  Could not resolve %s\n", domain)
		} else {
			fmt.Fprintf(w, "  Current records:\n")
			for _, ip := range aRecords {
				fmt.Fprintf(w, "    %s\n", ip)
			}
		}

		// Show required DNS records
		reqA := certInfo.DNSRequirements.A
		reqAAAA := certInfo.DNSRequirements.AAAA

		hasAllA := containsAll(aRecords, reqA)
		hasAllAAAA := containsAll(aRecords, reqAAAA)

		if hasAllA && hasAllAAAA {
			fmt.Fprintf(w, "  Status: OK\n")
		} else {
			fmt.Fprintf(w, "\n  ACTION REQUIRED:\n")
			for _, ip := range reqA {
				if !contains(aRecords, ip) {
					fmt.Fprintf(w, "    Add A record:    %s -> %s\n", domain, ip)
				}
			}
			for _, ip := range reqAAAA {
				if !contains(aRecords, ip) {
					fmt.Fprintf(w, "    Add AAAA record: %s -> %s\n", domain, ip)
				}
			}
		}
		fmt.Fprintln(w)

		// Show certificate details
		fmt.Fprintf(w, "TLS:\n")
		if len(certInfo.Certificates) == 0 {
			fmt.Fprintf(w, "  Pending (waiting for DNS validation)\n")
		} else {
			for _, c := range certInfo.Certificates {
				for _, issued := range c.Issued {
					expiry, _ := time.Parse(time.RFC3339, issued.ExpiresAt)
					daysLeft := int(time.Until(expiry).Hours() / 24)
					status := fmt.Sprintf("%d days remaining", daysLeft)
					if daysLeft < 7 {
						status = fmt.Sprintf("WARNING: expires in %d days!", daysLeft)
					}
					fmt.Fprintf(w, "  %s (%s) expires %s — %s\n",
						strings.ToUpper(issued.Type),
						issued.CertificateAuthority,
						expiry.Format("2006-01-02"),
						status,
					)
				}
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if i := strings.Index(host, ":"); i != -1 {
			host = host[:i]
		}

		if strings.HasPrefix(host, "www.") {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ok")
			return
		}

		scheme := "http"
		if r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		target := scheme + "://www." + host + r.URL.RequestURI()
		fmt.Printf("301 %s %s%s -> %s\n", r.RemoteAddr, host, r.URL.RequestURI(), target)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})

	fmt.Printf("Listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getOrCreateCert(apiBase, token, appName, domain string) (*flyCertResponse, bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Try to get existing certificate via REST API
	req, err := http.NewRequest("GET", apiBase+"/certificates/"+domain, nil)
	if err != nil {
		return nil, false, fmt.Errorf("invalid request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var cert flyCertResponse
		if err := json.NewDecoder(resp.Body).Decode(&cert); err != nil {
			return nil, false, fmt.Errorf("failed to parse certificate response: %v", err)
		}
		return &cert, false, nil
	}

	if resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("unexpected API response %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Certificate doesn't exist (404) — create it via GraphQL API
	gqlBody := fmt.Sprintf(
		`{"query":"mutation($appId: ID!, $hostname: String!) { addCertificate(appId: $appId, hostname: $hostname) { certificate { hostname } } }","variables":{"appId":%q,"hostname":%q}}`,
		appName, domain,
	)
	req, _ = http.NewRequest("POST", "https://api.fly.io/graphql", strings.NewReader(gqlBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create certificate: %v", err)
	}
	defer resp2.Body.Close()

	var gqlResp struct {
		Errors []struct{ Message string } `json:"errors"`
	}
	respBody, _ := io.ReadAll(resp2.Body)
	json.Unmarshal(respBody, &gqlResp)
	if len(gqlResp.Errors) > 0 {
		return nil, false, fmt.Errorf("failed to create certificate: %s", gqlResp.Errors[0].Message)
	}

	// Fetch the newly created certificate details via REST
	req, err = http.NewRequest("GET", apiBase+"/certificates/"+domain, nil)
	if err != nil {
		return nil, true, fmt.Errorf("invalid request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp3, err := client.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("certificate created but failed to fetch details: %v", err)
	}
	defer resp3.Body.Close()

	var cert flyCertResponse
	if err := json.NewDecoder(resp3.Body).Decode(&cert); err != nil {
		return nil, true, fmt.Errorf("certificate created but failed to parse details: %v", err)
	}
	return &cert, true, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsAll(haystack, needles []string) bool {
	for _, n := range needles {
		if !contains(haystack, n) {
			return false
		}
	}
	return true
}
