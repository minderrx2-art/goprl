# GO URL Shortener
Ultimate goal is to design and produce a scalable, production ready URL shortener, and learn some GO along the way.

Features so far implemented:
- [x] Add API
- [x] Add database
- [x] Add cache
- [x] Add rate limiting

Features still being worked on:
- [ ] Add analytics
- [ ] Add reverse proxy
- [ ] Add load balancing


Run via docker (recommended)
```
docker compose up --build
```

Run raw (Requires SQL + REDIS setup)
```
go run main.go 
```

Run unit tests via:
```
go test ./...
```

Testing URL shortening:
```
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.google.com"}'
```
Testing redirection via browser:
```
http://localhost:8080/abc123
```
To test within GCP (requires google cloud CLI + account)
```
cd terraform
terraform apply
```

You will receive back an IP once the infrastructure is set up which can then substitute the localhost in examples above.
