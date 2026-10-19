# Source verification

On 19 October 2026 UTC, 87 Python tests, `go test ./...`, strict validation of this OpenSpec change and `git diff --check` passed. Tests covered rejected public-origin/NodePort/CIDR combinations, a CloudFront-matching certificate SAN, UI-only NodePort rendering and scoped UI ingress. AWS route deployment and browser/runtime acceptance remain pending.
