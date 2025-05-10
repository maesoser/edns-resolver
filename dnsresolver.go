package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const EDE_DATA_OPT = 65050

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
)

// Config represents the DNS resolver configuration
type Config struct {
	ListenAddr  string
	Port        int
	MetricsPort int
	Upstream    string
	CacheSize   int
	RestURL     string
	DefaultTTL  uint32
}

// DNSResolver handles the DNS resolution
type DNSResolver struct {
	config  Config
	cache   *lru.Cache
	client  *dns.Client
	metrics *DNSMetrics
}

// CategoryResponse is the structure expected from the REST service
type CategoryResponse struct {
	Categories []int `json:"categories"`
}

// getCacheKey generates a cache key from a DNS request
func getCacheKey(req *dns.Msg) string {
	if len(req.Question) == 0 {
		return ""
	}
	q := req.Question[0]
	return fmt.Sprintf("%s-%d-%d", q.Name, q.Qtype, q.Qclass)
}

// NewDNSResolver creates a new DNS resolver with the given configuration
func NewDNSResolver(config Config) (*DNSResolver, error) {
	cache, err := lru.New(config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %v", err)
	}

	metrics := initializeMetrics()

	return &DNSResolver{
		config: config,
		cache:  cache,
		client: &dns.Client{
			Net:          "udp",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		metrics: metrics,
	}, nil
}

// Start starts the DNS resolver server
func (r *DNSResolver) Start() error {

	// Start Prometheus metrics endpoint
	metricsPort := getEnvAsInt("METRICS_PORT", r.config.MetricsPort)
	metricsAddr := fmt.Sprintf("%s:%d", r.config.ListenAddr, metricsPort)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("Starting metrics server on %s", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, nil); err != nil {
			log.Printf("Error starting metrics server: %v", err)
		}
	}()

	addr := fmt.Sprintf("%s:%d", r.config.ListenAddr, r.config.Port)
	server := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: dns.HandlerFunc(r.handleDNSRequest),
	}

	log.Printf("Starting DNS resolver on %s", addr)
	log.Printf("Using upstream resolver: %s", r.config.Upstream)
	log.Printf("Using REST service: %s", r.config.RestURL)

	return server.ListenAndServe()
}

// handleDNSRequest processes incoming DNS requests
func (r *DNSResolver) handleDNSRequest(w dns.ResponseWriter, req *dns.Msg) {
	startTime := time.Now()

	// Create labels for metrics
	var qtype, qclass, domainSuffix string
	if len(req.Question) > 0 {
		q := req.Question[0]
		qtype = dns.TypeToString[q.Qtype]
		qclass = dns.ClassToString[q.Qclass]

		// Extract domain suffix (last two parts of domain)
		domainParts := dns.SplitDomainName(q.Name)
		if len(domainParts) >= 2 {
			domainSuffix = domainParts[len(domainParts)-2] + "." + domainParts[len(domainParts)-1] + "."
		} else {
			domainSuffix = q.Name
		}

		r.metrics.RequestsTotal.WithLabelValues(qtype, qclass, domainSuffix).Inc()
	}

	log.Printf("Received query for: %s from %s", req.Question[0].Name, w.RemoteAddr())

	// Check cache first
	cacheKey := getCacheKey(req)
	if cached, ok := r.cache.Get(cacheKey); ok {
		cachedMsg := cached.(*dns.Msg).Copy()
		cachedMsg.Id = req.Id // Use the original request ID
		r.metrics.CacheHits.Inc()
		log.Printf("Cache hit for %s", req.Question[0].Name)
		// Record response code in metrics
		r.metrics.ResponseCodes.WithLabelValues(dns.RcodeToString[cachedMsg.Rcode]).Inc()

		// Record latency
		r.metrics.RequestLatency.WithLabelValues("cached").Observe(time.Since(startTime).Seconds())

		w.WriteMsg(cachedMsg)
		return
	} else {
		r.metrics.CacheMisses.Inc()
	}

	// Get client IP address
	sourceIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		log.Printf("Error getting source IP: %v", err)
		r.metrics.RequestLatency.WithLabelValues("error").Observe(time.Since(startTime).Seconds())
		dns.HandleFailed(w, req)
		return
	}

	// Prepare a modified request with EDNS options
	modifiedReq := req.Copy()
	r.addEDNSSubnet(modifiedReq, sourceIP)
	// Fetch categories from REST service
	restStartTime := time.Now()
	categoryResponse, err := r.fetchCategoriesFromREST(sourceIP)
	r.metrics.RestLatency.Observe(time.Since(restStartTime).Seconds())

	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		r.metrics.RestErrors.Inc()
		// Continue without adding EDNS options if REST service fails
	} else if len(categoryResponse.Categories) > 0 {
		// Add EDNS options
		r.addEDNSCategory(modifiedReq, categoryResponse)
		r.metrics.EdnsOptionsAdded.Inc()
		for _, cat := range categoryResponse.Categories {
			r.metrics.CategoriesReceived.WithLabelValues(strconv.Itoa(cat)).Inc()
		}
	}

	println(modifiedReq.String())

	// Forward the query to upstream resolver
	upstreamStartTime := time.Now()
	resp, _, err := r.client.Exchange(modifiedReq, r.config.Upstream)
	upstreamLatency := time.Since(upstreamStartTime).Seconds()
	r.metrics.UpstreamLatency.Observe(upstreamLatency)
	if err != nil {
		log.Printf("Error querying upstream DNS: %v", err)
		r.metrics.UpstreamErrors.Inc()
		r.metrics.RequestLatency.WithLabelValues("failed").Observe(time.Since(startTime).Seconds())
		dns.HandleFailed(w, req)
		return
	}

	// Set the original request ID
	resp.Id = req.Id

	// Record response code in metrics
	r.metrics.ResponseCodes.WithLabelValues(dns.RcodeToString[resp.Rcode]).Inc()

	// Cache the response
	r.cacheResponse(cacheKey, resp)

	// Send response to client
	w.WriteMsg(resp)

	// Record total latency
	r.metrics.RequestLatency.WithLabelValues("success").Observe(time.Since(startTime).Seconds())
}

// addEDNSOptions adds EDNS options to the DNS request
func (r *DNSResolver) addEDNSSubnet(req *dns.Msg, ipAddr string) {

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return // Return unmodified if IP is invalid
	}

	var opt *dns.OPT
	for _, rr := range req.Extra {
		if rr.Header().Rrtype == dns.TypeOPT {
			opt = rr.(*dns.OPT)
			break
		}
	}

	if opt == nil {
		opt = new(dns.OPT)
		opt.Hdr.Name = "."
		opt.Hdr.Rrtype = dns.TypeOPT
		opt.Hdr.Class = dns.DefaultMsgSize
		opt.Hdr.Ttl = 0
		req.Extra = append(req.Extra, opt)
	}

	// Add our EDNS option
	option := &dns.EDNS0_SUBNET{
		Code: dns.EDNS0SUBNET,
	}
	// Determine if IPv4 or IPv6
	if ip.To4() != nil {
		// IPv4
		option.Family = 1         // 1 for IPv4 source address
		option.SourceNetmask = 32 // Full /32 netmask for specific IP
		option.SourceScope = 0    // RFC7871 recommendation
		option.Address = ip.To4() // Use IPv4 format
	} else {
		// IPv6
		option.Family = 2          // 2 for IPv6 source address
		option.SourceNetmask = 128 // Full /128 netmask for specific IP
		option.SourceScope = 0     // RFC7871 recommendation
		option.Address = ip        // Use IPv6 format
	}
	opt.Option = append(opt.Option, option)
}

// addEDNSOptions adds EDNS options to the DNS request
func (r *DNSResolver) addEDNSCategory(req *dns.Msg, categories CategoryResponse) {

	// Serialize to JSON
	jsonData, err := json.Marshal(categories)
	if err != nil {
		log.Printf("Error serializing EDE data: %v", err)
		return
	}

	// Create an OPT record if it doesn't exist
	var opt *dns.OPT
	for _, rr := range req.Extra {
		if rr.Header().Rrtype == dns.TypeOPT {
			opt = rr.(*dns.OPT)
			break
		}
	}

	if opt == nil {
		opt = new(dns.OPT)
		opt.Hdr.Name = "."
		opt.Hdr.Rrtype = dns.TypeOPT
		opt.Hdr.Class = dns.DefaultMsgSize
		opt.Hdr.Ttl = 0
		req.Extra = append(req.Extra, opt)
	}

	// Add our EDNS option
	option := &dns.EDNS0_LOCAL{
		Code: EDE_DATA_OPT,
		Data: jsonData,
	}
	opt.Option = append(opt.Option, option)

	log.Printf("Added EDNS option with categories: %v", categories)
}

// fetchCategoriesFromREST fetches categories from the REST service
func (r *DNSResolver) fetchCategoriesFromREST(sourceIP string) (CategoryResponse, error) {
	var categoryResp CategoryResponse
	url := fmt.Sprintf("%s/%s", r.config.RestURL, sourceIP)

	// Set timeout for HTTP request
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return categoryResp, fmt.Errorf("failed to query REST service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return categoryResp, fmt.Errorf("REST service returned non-OK status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return categoryResp, fmt.Errorf("failed to read REST response: %v", err)
	}

	if err := json.Unmarshal(body, &categoryResp); err != nil {
		return categoryResp, fmt.Errorf("failed to parse REST response: %v", err)
	}

	return categoryResp, nil
}

// cacheResponse caches a DNS response
func (r *DNSResolver) cacheResponse(key string, msg *dns.Msg) {
	// Only cache successful responses
	if msg.Rcode != dns.RcodeSuccess {
		return
	}

	// Find minimum TTL
	minTTL := r.config.DefaultTTL
	for _, answer := range msg.Answer {
		if answer.Header().Ttl < minTTL {
			minTTL = answer.Header().Ttl
		}
	}

	// Cache the copy of the message
	r.cache.Add(key, msg.Copy())

	// Schedule cache expiration
	if minTTL > 0 {
		time.AfterFunc(time.Duration(minTTL)*time.Second, func() {
			r.cache.Remove(key)
		})
	}
}
