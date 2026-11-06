## zap
- main: main
- prod: none
- test: CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test -count=1 ./... && npm --prefix desktop/frontend test -- --run && bash scripts/test-scripts.sh && bash scripts/check-docs.sh
