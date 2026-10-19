# Proposal

## Why

The AWS lab needs an AWS-assigned HTTPS URL for the SPIRE web UI. The existing shared deployment binds the browser certificate and origin to loopback, and its UI Service has no ALB target port.

## What changes

Allow an explicitly pinned CloudFront HTTPS origin in the shared deployment. Generate the private backend browser certificate for that hostname, expose only the Ready UI through a fixed NodePort, allow ingress from the ALB subnets, and keep local loopback verification usable. Platform owns CloudFront and ALB; this PoC owns only its UI Service, policy, certificate and validation.

## Impact

The fake-money UI remains an assumed-operator demo with exact Host, Origin and CSRF checks. Money and callers stay private; their SVID signatures and mesh mTLS are unchanged.
