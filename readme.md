# GO URL Shortener
Ultimate goal is to design and produce a scalable, production ready URL shortener.

Features Implemented:
- [x] Add API
- [x] Add database
- [x] Add cache
- [x] Add rate limiting
- [x] CI/CD

Required env variables
```
DATABASE_URL="postgres://{user}:{REDIS_PASSWORD}@{host}:{port}/{db_name}"
REDIS_URL="redis://:{password}@{host}:{port}/0"
```
Optional (Required for docker)
```
REDIS_PASSWORD={password}
DB_PASSWORD={password}
RATE_LIMIT={number}
```

Run via docker (recommended)
```
docker compose up --build
```

Run unit tests via:
```
go test ./...
```

Testing URL shortening:
```
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"http://www.goprl.co.uk"}'
```
Testing redirection via browser:
```
http://www.goprl.co.uk/abc123
```
To test within GCP (requires google cloud CLI + account)
```
cd terraform
terraform apply
```

You will receive back an IP once the infrastructure is set up which can then substitute the localhost in examples above.

---
## K6 stress test results
#### URL shortening
```bash
    scenarios: (100.00%) 1 scenario, 10 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 10 looping VUs for 2m0s over 3 stages (gracefulRampDown: 30s, gracefulStop: 30s)

  █ THRESHOLDS 

    http_req_duration
    ✓ 'p(99)<3000' p(99)=102.66ms


  █ TOTAL RESULTS 

    checks_total.......: 6592    54.905625/s
    checks_succeeded...: 100.00% 6592 out of 6592
    checks_failed......: 0.00%   0 out of 6592

    ✓ 201

    HTTP
    http_req_duration..............: avg=99.95ms  min=97.79ms med=99.77ms max=123.28ms p(90)=101.18ms p(95)=101.57ms
      { expected_response:true }...: avg=99.95ms  min=97.79ms med=99.77ms max=123.28ms p(90)=101.18ms p(95)=101.57ms
    http_req_failed................: 0.00%  0 out of 6592
    http_reqs......................: 6592   54.905625/s

    EXECUTION
    iteration_duration.............: avg=100.16ms min=97.84ms med=99.83ms max=201.82ms p(90)=101.23ms p(95)=101.64ms
    iterations.....................: 6592   54.905625/s
    vus............................: 1      min=1         max=10
    vus_max........................: 10     min=10        max=10

    NETWORK
    data_received..................: 1.8 MB 15 kB/s
    data_sent......................: 1.1 MB 8.9 kB/s

```
#### Redirection
```
     scenarios: (100.00%) 1 scenario, 10 max VUs, 2m30s max duration (incl. graceful stop):
              * default: Up to 10 looping VUs for 2m0s over 3 stages (gracefulRampDown: 30s, gracefulStop: 30s)



  █ TOTAL RESULTS 

    checks_total.......: 6718    51.61095/s
    checks_succeeded...: 100.00% 6718 out of 6718
    checks_failed......: 0.00%   0 out of 6718

    ✓ 301

    HTTP
    http_req_duration..............: avg=98.14ms min=96.33ms med=98.1ms  max=133.86ms p(90)=99.09ms p(95)=99.31ms
      { expected_response:true }...: avg=98.14ms min=96.33ms med=98.1ms  max=133.86ms p(90)=99.09ms p(95)=99.31ms
    http_req_failed................: 0.00%  0 out of 6818
    http_reqs......................: 6818   52.379199/s

    EXECUTION
    iteration_duration.............: avg=98.31ms min=96.37ms med=98.13ms max=198.53ms p(90)=99.12ms p(95)=99.34ms
    iterations.....................: 6718   51.61095/s
    vus............................: 1      min=0         max=10
    vus_max........................: 10     min=10        max=10

    NETWORK
    data_received..................: 2.1 MB 16 kB/s
    data_sent......................: 494 kB 3.8 kB/s
```
