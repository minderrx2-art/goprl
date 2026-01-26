# GO URL Shortener
Project description: Ultimate goal is to practice systems design concepts and produce a scalable, production ready URL shortener.

High level todo list:
- [x] Add API
- [x] Add database
- [x] Add cache
- [ ] Add rate limiting
- [ ] Add reverse proxy
- [ ] Add load balancing
- [ ] Add analytics

Maybe:
- [ ] Create Kubernetes cluster staging/production

Testing URL shortening:
```
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.google.com"}'
```
Testing redirection:
```
curl -L http://localhost:8080/abc123
```