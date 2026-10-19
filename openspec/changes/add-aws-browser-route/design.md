# Design

The shared target may provide `spire.ui_origin` as an exact AWS `https://*.cloudfront.net` URL, `ui_node_port: 30443`, and private ALB subnet CIDRs. Validation rejects partial or non-AWS browser route input before mutation. The UI's generated browser certificate contains the origin hostname SAN, while the Service exposes only port 8443 on the fixed NodePort with local-only endpoints. A namespace NetworkPolicy admits only the configured ALB subnet addresses to UI port 8443; the platform node security group restricts the NodePort to the ALB security group.

Loopback acceptance connects to the forwarded UI while sending the public Host and TLS SNI. It opens the UI page first to establish the browser session cookie before calling the session API, then checks signed Money and CSRF behavior. Without the optional public configuration, the shared lab keeps its current loopback origin and private Service.

The UI's dedicated browser HTTPS listener accepts TLS 1.2 or newer because the lab ALB's HTTP listener uses AWS's TLS 1.2 backend connection policy. This does not alter Istio's SPIRE-issued TLS 1.3 mesh traffic or Kafka TLS. A fresh image and owned run are required because replacing Pods in the existing run would invalidate its original-Pod guard.
