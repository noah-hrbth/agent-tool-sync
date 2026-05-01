# Security policy

## Reporting a vulnerability

Please report security vulnerabilities privately via GitHub's [private vulnerability reporting](https://github.com/noah-hrbth/agent-tool-sync/security/advisories/new).

Do **not** open public GitHub issues for security reports.

We aim to acknowledge reports within 72 hours and ship a fix as soon as practical.

## Verifying release artifacts

Releases publish a `checksums.txt` file. Starting with the next tagged release this file is signed with [cosign](https://github.com/sigstore/cosign) using GitHub's OIDC identity, and an SBOM (syft) is attached to each archive.

Verify a download:

```sh
# Verify the checksum file's signature.
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature  checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/noah-hrbth/agent-tool-sync/.+' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

# Verify your downloaded archive against the checksum file.
sha256sum --check --ignore-missing checksums.txt
```
