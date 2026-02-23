package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "1234"
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	ipv4 := os.Getenv("FLY_PUBLIC_IP_V4")
	if ipv4 == "" {
		ipv4 = "66.241.124.202"
	}
	ipv6 := os.Getenv("FLY_PUBLIC_IP_V6")
	if ipv6 == "" {
		ipv6 = "2a09:8280:1::d7:f561:0"
	}

	http.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/status/")
		domain = strings.TrimSpace(domain)
		if domain == "" {
			http.Error(w, "Usage: /status/<domain>", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		fmt.Fprintf(w, "=== %s ===\n\n", domain)

		// Check A records
		aRecords, err := net.LookupHost(domain)
		if err != nil {
			fmt.Fprintf(w, "DNS: could not resolve %s\n\n", domain)
		} else {
			fmt.Fprintf(w, "DNS records found:\n")
			for _, ip := range aRecords {
				fmt.Fprintf(w, "  %s\n", ip)
			}
			fmt.Fprintln(w)
		}

		// Check what's needed
		hasIPv4 := false
		hasIPv6 := false
		for _, ip := range aRecords {
			if ip == ipv4 {
				hasIPv4 = true
			}
			if ip == ipv6 {
				hasIPv6 = true
			}
		}

		if hasIPv4 && hasIPv6 {
			fmt.Fprintf(w, "OK: DNS is correctly configured\n")
		} else {
			fmt.Fprintf(w, "ACTION REQUIRED:\n")
			if !hasIPv4 {
				fmt.Fprintf(w, "  Add A record:    %s -> %s\n", domain, ipv4)
			}
			if !hasIPv6 {
				fmt.Fprintf(w, "  Add AAAA record: %s -> %s\n", domain, ipv6)
			}
		}

		// Check TLS certificate
		fmt.Fprintf(w, "\nCertificate:\n")
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 5 * time.Second},
			"tcp", domain+":443",
			&tls.Config{ServerName: domain},
		)
		if err != nil {
			fmt.Fprintf(w, "  No valid certificate (fly certs add %s)\n", domain)
		} else {
			defer conn.Close()
			cert := conn.ConnectionState().PeerCertificates[0]
			fmt.Fprintf(w, "  Issuer:  %s\n", cert.Issuer.Organization)
			fmt.Fprintf(w, "  Expiry:  %s\n", cert.NotAfter.Format("2006-01-02"))
			daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
			if daysLeft < 7 {
				fmt.Fprintf(w, "  WARNING: expires in %d days!\n", daysLeft)
			} else {
				fmt.Fprintf(w, "  Valid:   %d days remaining\n", daysLeft)
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
