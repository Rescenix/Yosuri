# Code signing policy

## Status

ResceneAgent is applying for the SignPath Foundation open-source code-signing program. Until that application is approved and the signing workflow is configured, existing release files may be unsigned.

Planned signing provider: **Free code signing provided by SignPath.io, certificate by SignPath Foundation**.

## Release controls

- Windows installers are built from this public repository by GitHub-hosted GitHub Actions runners.
- The workflow uploads the unsigned installer as a GitHub Actions artifact before submitting it to SignPath.
- A ResceneAgent signing approver must manually approve every production signing request.
- A tagged GitHub Release is published only after the returned installer passes Windows Authenticode verification.
- Release checksums are calculated after signing and published alongside the installer.
- The release workflow does not publish portable ZIP files or unsigned installers as GitHub Release assets.

## Team roles

- **Committers and reviewers:** members with write access to the [Rescenix organization](https://github.com/Rescenix).
- **Signing approvers:** administrators of the [ResceneAgent repository](https://github.com/Rescenix/ResceneAgent).

Repository administrators are responsible for protecting the SignPath signing policy, reviewing release changes, and denying any signing request that cannot be traced to an approved repository build.

## Repository configuration after approval

The release workflow intentionally fails before signing or publishing a tagged release until these GitHub Actions settings exist:

- secret `SIGNPATH_API_TOKEN`;
- variable `SIGNPATH_ORGANIZATION_ID`;
- variable `SIGNPATH_PROJECT_SLUG`;
- variable `SIGNPATH_SIGNING_POLICY_SLUG`.

Before approval, maintainers can run the `Windows release` workflow manually with `unsigned_only` enabled. This produces the unsigned installer artifact needed for SignPath onboarding without creating a GitHub Release.

## Privacy and network disclosure

ResceneAgent stores workspace state and local configuration on the user's computer. Network connections occur when the user invokes or configures network-backed functionality, including:

- model providers selected by the user;
- GitHub release checks and hosted registry content;
- optional ResceneCloud authentication, account synchronization, and memory backup;
- websites or APIs explicitly accessed by an agent workflow.

API keys and other credentials must not be included in release artifacts or build logs. The code-signing workflow submits only the compiled Windows installer to SignPath; it does not submit user data, workspace files, or local configuration.

## Verifying a release

On Windows, verify the Authenticode signature and SHA-256 checksum before installation:

```powershell
Get-AuthenticodeSignature .\ResceneAgent-windows-amd64-setup.exe
Get-FileHash .\ResceneAgent-windows-amd64-setup.exe -Algorithm SHA256
```

For a signed production release, `Get-AuthenticodeSignature` must report `Valid`, and the calculated hash must match `SHA256SUMS.txt` from the same GitHub Release.
