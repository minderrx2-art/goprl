# GO URL Shortener
High level todo list:
- [ ] Add API
- [ ] Add database
- [ ] Add cache
- [ ] Add rate limiting
- [ ] Add reverse proxy
- [ ] Add load balancing
- [ ] Add analytics

Maybe:
- [ ] Create Kubernetes cluster staging/production

Assumptions to keep in mind:
- 1–5k steady, 20k burst requests per second
- Read-heavy (≈95% GET / redirect)
- Writes are rare but must be consistent
- Latency target: p95 < 50ms for redirect
- Data loss tolerance: near-zero for mappings
- Analytics can be eventually consistent

Project description: Ultimate goal is to practice systems design concepts and produce a scalable, production ready URL shortener.

Testing:
```
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.google.com"}'
```

```
curl -L http://localhost:8080/abc123
```