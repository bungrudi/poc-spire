## Context

The owner approved “One shared foundation repository, with deployments owned by each PoC” and explicitly requested implementation on 15 October 2026 in this KKI chat.

## Goals / Non-Goals

Provide one reusable Sydney EKS foundation and two PoC adapters. Preserve existing local and public runtimes. No GitOps controller, public ingress, production sizing or automatic account credential creation.

## Decisions

Terraform exclusively owns AWS resources; the platform installs shared Istio and passes explicit target/configuration inputs to PoC adapters. Component definitions stay in their source repositories. Pin commits and image digests; reject unresolved pins before deployment. Use two small managed x86 Spot nodes, public-subnet egress without NAT, a restricted EKS API CIDR and private Kubernetes services. S3 telemetry is disposable; the separately bootstrapped state backend survives lab teardown.

PoC adapters validate kube-system UID, API endpoint, CA fingerprint and a platform identity marker. They reject namespace adoption and cluster-resource collisions. Removal is limited to recorded namespace UIDs and exact owned cluster-resource names. SPIRE retains executable/Pod/owner/image conjunctions and its monotonic run guard; it never installs Istio or changes kubelet configuration. Telemetry integration initially collects mesh TCP metrics; application OTLP instrumentation remains an explicit future capability.

## Risks / Trade-offs

Fresh SPIRE runs reset identity and fake ledger state. Export sanitized evidence before removal. Full EKS acceptance requires AWS access and runtime capacity; static validation cannot establish deployment acceptance.

## Migration Plan

Add new entry points without invoking them against existing clusters. Validate rendering and negative safety cases, then deploy a disposable lab and run positive, negative, removal-isolation and recreation checks.

## Approved Sydney adjustment

Use only account 581695527661 and region ap-southeast-2 for this shared lab. Align the Terraform provider, backend, separate state bootstrap and all PoC target guards; reject the former Jakarta target. Do not upgrade the account, activate advanced features or alter the observability production region. Validate positive Sydney and negative Jakarta/account cases before AWS provisioning.

## Spot worker design

Use two EKS managed Spot node groups, named spire and observability, each with desired/minimum/maximum size one and only m7i-flex.large (x86_64, 2 vCPU, 8 GiB). AWS rejected the earlier 32-GiB candidates because they are not eligible under this Free Plan; the live EC2 eligibility API reports m7i-flex.large as eligible. Keep both workers in the existing availability zone and use encrypted 40-GiB gp3 root disks. Label both with poc-platform/lab and each with poc-platform/workload matching its group. SPIRE selects exactly one Ready node labelled for spire; all its workloads retain that selector and the existing observed-node identity binding. Observability may use shared spare capacity. This 16-GiB aggregate starting capacity needs runtime fit checks; do not claim the original 32-GiB capacity or full acceptance. No On-Demand fallback is configured. Replacement/rebalancing can briefly overlap instances. Pricing varies and is not a spending cap.

Interruption may end an experiment. Node/Pod replacement must not relax SPIRE's identity or monotonic-run checks: quarantine and start a fresh run when required. EBS is zonal; this is not a highly available lab. Export evidence and destroy after sessions. The EKS control-plane hourly charge continues until cluster deletion.

## Free Plan workload identity

Use EKS Pod Identity instead of an IAM OIDC provider: the current account explicitly denies the latter through its service control policy. The platform owns the pinned Pod Identity agent, role associations and IAM trust policies bound to the cluster ARN, namespace and service account. EBS uses kube-system/ebs-csi-controller-sa; Loki and Tempo use separate observability service accounts and bucket-scoped roles. Do not disable session tags. The PoC removes IRSA annotations and static credentials, allowing supported AWS SDKs to use their default credential chains. Strict network policies allow TCP 80 only to the local agent address 169.254.170.23/32 in addition to existing API/S3 connectivity. Do not change the account policy; successful association and credential use remain runtime checks.

## SPIRE infrastructure connectivity on strict EKS

The shared adapter must install owned infrastructure policies before SPIRE startup. Permit DNS and Kubernetes API HTTPS for the server and agent, agent-to-server TCP 8081, and agent-to-selected-node kubelet TCP 10250. Resolve the selected node InternalIP from the checked cluster and scope kubelet egress to that /32. Do not grant application namespace access to these ports, change kubelet TLS verification, use host networking, or weaken application default-deny rules. This fills the connectivity required by the approved existing-cluster mode. AWS documents unreliable network-policy enforcement for directly created Pods: the existing guarded issuer is a standalone Pod, so do not claim its network isolation as accepted without a separate supported-controller design and tests. The application Deployments retain strict policies.
