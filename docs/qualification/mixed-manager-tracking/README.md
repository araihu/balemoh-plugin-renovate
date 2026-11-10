# Mixed-manager tracking regression

PR #866 exposed a mismatch between extraction and association. In `k8s/dex/restore-test.yaml`, the Kubernetes manager owns one `image:` reference while a custom regex manager owns an identical `imageName:` reference. Renovate created one commit with the approved Kubernetes digest update and an automatic inventory fingerprint. Both inventory checks passed. The original tracker rejected the PR because it required replacing every literal occurrence in the file.

The tracker now reconstructs the exact selected Kubernetes `image:` replacements when other identical references are present. The selected occurrence count, complete-reference boundaries and full-file byte comparison remain required. Other manager content, whitespace and comments cannot change. Mixed selectors not covered by the pinned Kubernetes line extractor remain rejected.

`TestMixedManagersRequireExactSelectedKubernetesReferences` covers tag, tagged digest, pinned tag and untagged digest cases, quoted/CRLF and embedded manifests, wrong-manager changes, extra edits and partial updates. `TestMixedManagerPRRecoversReadOnlyWithoutWriterReplay` verifies retained-request recovery without writes and rejects later unapproved content. The container regression compares the accepted selector shapes with the actual pinned Renovate 44.197.0 extractor without network access.

Local unit/race tests, vet and native extractor parity pass. Live revalidation of #866 requires deploying the corrected runtime. Upgrade PR merges stay manual.
