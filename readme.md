# DNS Resolver for Cloudflare Gateway EDNS Category Filtering

DNS resolver that supports Cloudflare Gateway's EDNS-based custom category filtering. This resolver enables device-level (or source IP level) filtering decisions by allowing clients to specify [which categories](https://developers.cloudflare.com/cloudflare-one/policies/gateway/domain-categories/) they wish to block through EDNS options embedded in DNS requests.

## Disclaimer

This project is provided as a proof of concept (PoC) and is intended for experimental purposes only. It is not production-ready and should not be deployed in production environments.

The current implementation may have performance limitations, including (but not limited to):
- Use of HTTP for retrieving category information, which may introduce latency compared to more efficient mechanisms (e.g., in-memory caching, gRPC, or local databases).
- Lack of unit tests and automated test coverage.
- Limited or incomplete logging, monitoring, and error handling.

Before considering this resolver for production use, significant improvements in performance, testing, stability, and observability would be required.

## Key Features

- **EDNS Category Filtering**: Embed category filters directly in DNS queries using EDNS options.
- **EDNS Client Subnet (ECS) compatible**: Attaches the client subnet to the DNS query.
- **LRU Cache**: High-performance caching to reduce load in upstream DNS resolvers.
- **Prometheus Metrics**: Comprehensive metrics for monitoring performance and behavior.
- **Configurable**: Easily configurable through environment variables.

## How It Works

When a client sends a DNS query, the resolver performs the following steps:

1. The resolver receives a DNS query from a client
2. It extracts the client's source IP address
3. It calls a configurable REST service with the client's IP to determine which category filters to apply
4. The REST service returns an array of category IDs
5. The resolver adds these category IDs as EDNS options in the DNS query
6. The resolver forwards the modified query to Cloudflare Gateway
7. Cloudflare Gateway applies filtering based on the embedded category IDs
8. The resolver caches the response according to TTL values
9. The resolver returns the response to the client

```
                               ┌─────────────────┐                                  
                               │                 │                                  
                               │ REST Service    │                                  
                               │                 │                                  
                               └────────┬────────┘                                  
                                       ▲│                                           
                                       ││ HTTP/Protobuff                            
                                       │▼                                           
                               ┌───────┴─────────┐           ┌─────────────────────┐
                               │                 │           │                     │
 ┌─────────────┐     DNS       │                 │    DNS    │  Cloudflare         │
 │ DNS Client  │◄─────────────►│  edns-resolver  │◄─────────►│  Gateway            │
 └─────────────┘               │                 │           │                     │
                               │                 │           │                     │
                               └───────┬─────────┘           └─────────────────────┘
                                       │                                            
                                       │ HTTP/Protobuff                             
                                       ▼                                            
                               ┌─────────────────┐                                  
                               │                 │                                  
                               │ Logging service │                                  
                               │                 │                                  
                               └─────────────────┘                                  
                                                                                    
```

## Getting Started

### Prerequisites

- Go 1.16 or higher
- Access to a Cloudflare Gateway DNS resolver
- A REST service that can determine category IDs based on IP addresses

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/maesoser/edns-resolver.git
   cd cf-edns-resolver
   ```

2. Install dependencies:
   ```bash
   go get github.com/miekg/dns
   go get github.com/hashicorp/golang-lru
   go get github.com/prometheus/client_golang/prometheus
   ```

   or 

   ```
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o edns-resolver
   ```

### Configuration

The resolver can be configured using the following environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DNS_LISTEN_ADDR` | Address to listen on | `0.0.0.0` |
| `DNS_PORT` | Port to listen for DNS requests | `5053` |
| `DNS_UPSTREAM` | Upstream DNS resolver (Cloudflare Gateway) | `1.0.0.1:53` |
| `DNS_CACHE_SIZE` | Size of the LRU cache | `1024` |
| `DNS_REST_URL` | URL of the REST service for category lookup | `http://localhost:8080/categories` |
| `DNS_DEFAULT_TTL` | Default TTL for cached entries (seconds) | `60` |
| `METRICS_PORT` | Port for Prometheus metrics | `9100` |

### Running

```bash
# Basic usage
./edns-resolver

# With custom configuration
DNS_PORT=5353 DNS_UPSTREAM=1.1.1.2:53 ./edns-resolver
```

## REST Service Integration

The resolver expects the REST service to return a JSON response in this format:

```json
{
  "categories": [29, 30, 31]
}
```

Where each number in the array represents a Cloudflare Gateway category ID.

Example REST service endpoint:
```
GET http://your-rest-service/categories/${ipaddr}
```

We included a test webserver (`webserver.py`) in this repo.

## Metrics

The resolver exposes Prometheus metrics on port 9153 (configurable). Available metrics include:

### Request & Response Metrics
- `dns_requests_total`: Total number of DNS requests received
- `dns_request_latency_seconds`: End-to-end latency of DNS request processing
- `dns_response_codes_total`: Distribution of DNS response codes

### Cache Metrics
- `dns_cache_hits_total`: Number of cache hits
- `dns_cache_misses_total`: Number of cache misses
- `dns_cache_size`: Current number of entries in the cache

### REST Service Metrics
- `dns_rest_latency_seconds`: Latency of REST service queries
- `dns_rest_errors_total`: Number of REST service errors
- `dns_categories_received_total`: Categories received from REST service

### Upstream Resolver Metrics
- `dns_upstream_latency_seconds`: Latency of upstream DNS resolver queries
- `dns_upstream_errors_total`: Number of upstream DNS resolver errors

### EDNS Options Metrics
- `dns_edns_options_added_total`: Number of EDNS options added to queries

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

- [Cloudflare Gateway](https://developers.cloudflare.com/cloudflare-one/policies/filtering/dns-policies/) for the Gateway DNS filtering system
- [dns](https://github.com/miekg/dns) Go package for DNS protocol implementation
- [golang-lru](https://github.com/hashicorp/golang-lru) for the LRU cache implementation
- [Prometheus](https://prometheus.io/) for metrics collection