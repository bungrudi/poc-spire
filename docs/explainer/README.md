# SPIRE process identity guide

This static page follows one fake dollar through the UI, caller, SPIRE Agent,
SPIRE Server and Money service. It includes four rendered Mermaid scenarios
and six excerpts from this repository's Go sender and receiver.

Open [the hosted guide](https://spire-process-identity.rudiadianto.chatgpt.site/)
with its existing Sites access. This directory is a portable export of that
page, including the Mermaid definitions and generated SVGs. It has no build
step or runtime Mermaid dependency.

To preview from the repository root:

```sh
python3 -m http.server 8080 --bind 127.0.0.1 --directory docs/explainer
```

Then open `http://127.0.0.1:8080/`. The page simulates outcomes; it does not call
the live transfer API. The Go excerpts omit surrounding error handling where
the captions say so. SVGs were generated using Mermaid 12.1.0; changing a diagram
requires regenerating both its standalone SVG and the embedded HTML copy.

The hosted Sites checkout is separate and ignored at `/sites/`. When updating
that page, copy its checked static assets here before publishing to GitHub.
Keep all credentials, protected runtime state and temporary tooling out of
this export.
