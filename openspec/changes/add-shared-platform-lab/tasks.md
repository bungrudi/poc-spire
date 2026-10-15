## 1. Implementation
- [x] 1.1 Implement approved repository boundaries and explicit target/configuration interfaces.
- [x] 1.2 Add resource ownership, immutable-source and cleanup guards.
- [x] 1.3 Add meaningful deterministic tests and deployment runbooks.

## 2. Verification
- [x] 2.1 Validate OpenSpec, Terraform, rendering and negative guard cases.
- [ ] 2.2 Run integrated runtime, telemetry, isolation, teardown and clean-recreation acceptance on a disposable lab.

## 3. Approved Sydney adjustment
- [x] 3.1 Align shared-lab configuration and explicit account/region guards with account 581695527661 in Sydney ap-southeast-2; preserve separate production targets.
- [x] 3.2 Validate Sydney acceptance, Jakarta/account rejection and strict OpenSpec consistency; keep AWS runtime acceptance separate.

## 6. Free Plan compatibility correction
- [ ] 6.1 Adapt owned configuration to two eligible 8-GiB Spot workers, preserve the SPIRE single selected node contract and use scoped EKS Pod Identity.
- [ ] 6.2 Validate ownership, target, compute and credential-boundary checks; record real AWS results separately from integrated runtime acceptance.

## 7. Strict EKS SPIRE connectivity
- [ ] 7.1 Add namespace-owned infrastructure rules before startup, with kubelet egress bound to the verified selected node address.
- [ ] 7.2 Validate policy scope and actual startup separately; record the standalone issuer policy-enforcement limitation.
