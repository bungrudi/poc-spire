# Design

The shared target may provide `spire.ui_origin` as an exact AWS `https://*.cloudfront.net` URL, `ui_node_port: 30443`, and private ALB subnet CIDRs. Validation rejects partial or non-AWS browser route input before mutation. The UI's generated browser certificate contains the origin hostname SAN, while the Service exposes only port 8443 on the fixed NodePort with local-only endpoints. A namespace NetworkPolicy admits only the configured ALB subnet addresses to UI port 8443; the platform node security group restricts the NodePort to the ALB security group.

Loopback acceptance connects to the forwarded UI while sending the public Host and TLS SNI, so its existing browser, signed Money and CSRF assertions remain meaningful. Without the optional public configuration, the shared lab keeps its current loopback origin and private Service.
