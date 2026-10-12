# Kafka signing references and pins

The [normative profile](kafka-signing-profile.md) defines this application's
fields, bounds, identity policy and retry behavior. Standards below supply the
wire/identity building blocks; they do not define the PoC's payment semantics.

| Source | Used for |
| --- | --- |
| [RFC 7515](https://www.rfc-editor.org/rfc/rfc7515.html), especially §§3.1,4.1.6 and Appendix A.3 | Compact JWS, exact protected signing input, x5c DER/base64 chain and ES256 vector. |
| [RFC 7518 §3.4](https://www.rfc-editor.org/rfc/rfc7518.html#section-3.4) | P-256/SHA-256 and 64-byte R||S signature encoding. |
| [RFC 8725](https://www.rfc-editor.org/rfc/rfc8725.html) | Fixed algorithm/curve, strict input, type/destination binding; this envelope is not a JWT. |
| [SPIFFE X.509-SVID](https://spiffe.io/docs/latest/spiffe-specs/x509-svid/) and [Go SPIFFE 2.6.0 verifier](https://github.com/spiffe/go-spiffe/blob/v2.6.0/svid/x509svid/verify.go) | Certificate-derived workload identity, current independent trust/path verification and SDK source rotation. |
| [SPIRE 1.14.0 workload keys](https://github.com/spiffe/spire/blob/v1.14.0/pkg/agent/workloadkey/workloadkey.go) | The measured native-agent ECP256 workload signing source. Executable/deployment attestation remains a conjunction. |
| [Kafka 4.0 TLS](https://kafka.apache.org/40/security/encryption-and-authentication-using-ssl/) and [ACLs](https://kafka.apache.org/40/security/authorization-and-acls/) | Stock mandatory client-certificate broker admission, hostname verification and default DN principals, separate from SPIFFE/JWS authorization. |
| [go-jose 4.1.5 release](https://github.com/go-jose/go-jose/releases/tag/v4.1.5), [franz-go 1.19.5](https://github.com/twmb/franz-go/releases/tag/v1.19.5), [jose 6.1.0](https://github.com/panva/jose/releases/tag/v6.1.0) | Pinned Go signing/transport and independent test-only JOSE implementation. |

Actual dependencies live in [go.mod](../go.mod) and
[package-lock.json](../package-lock.json). The stock JVM Kafka image is pinned in
[Compose](../infra/kafka-signing/compose.yaml) to the 4.0.2 multi-platform index
`sha256:836cafdad9f4825880d7cf1d5a21202915ae2527bd0ef1c3600c526ed7814d1f`.
Go 1.24.2, SPIRE 1.14.0, Go SPIFFE 2.6.0, go-jose 4.1.5, franz-go 1.19.5,
kmsg 1.11.2, Node 22.23.0 and jose 6.1.0 are the tested pins. Node/jose are used
only for development verification. No global installation is required.

The [dependency assessment](../openspec/changes/add-spiffe-kafka-message-signing/implementation.md)
records compatibility and relevant upstream security review. It is not a full
container/host vulnerability scan or a recommendation to run this learning
fixture as production payments. The approved historical draft and owner
manifest remain in the active change for audit; stable current contracts live
in [OpenSpec specs](../openspec/specs/).
