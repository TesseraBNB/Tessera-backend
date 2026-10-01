// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/**
 * @title TesseraAttestations
 * @notice Tamper-proof on-chain citation layer for Tessera verdicts.
 *
 * Tessera is an off-chain analytics agent; this contract adds an OPTIONAL,
 * PERMISSIONLESS registry on BNB Chain where anyone can commit the
 * keccak256 hash of a Tessera verdict (or any evidence document) plus a
 * pointer to its full content. The chain becomes the notary: a committed
 * hash proves the verdict existed at a point in time and was not altered
 * afterwards.
 *
 * Design constraints (deliberate):
 *  - No custody: the contract never holds funds or tokens.
 *  - No admin: no owner, no upgrade path, no pausing, no allowlist.
 *    Deployed once, immutable forever. Nothing to rug, nothing to govern.
 *  - Minimal data: hashes and pointers only. Verdict/report content lives
 *    off-chain (Tessera reports, PDFs, HTTP archives); the chain stores
 *    just enough to notarize it.
 *  - Open commit: anyone may commit. Trust rests on the hash preimage,
 *    not on the committer. Re-committing the same hash is a no-op.
 */
contract TesseraAttestations {
    // ─── Types ─────────────────────────────────────────────────────────────

    /// @notice Risk verdict classification, mirroring Tessera's analyst output.
    enum RiskLevel {
        LOW,     // 0
        MEDIUM,  // 1
        HIGH,    // 2
        UNKNOWN  // 3 — committed without a risk classification
    }

    struct Attestation {
        address committer;   // who notarized the preimage
        uint64 committedAt;  // block timestamp
        RiskLevel riskLevel; // verdict classification (informational)
        string projectId;    // e.g. "octant:ep-23:project-x" (informational)
        string evidenceUri;  // pointer to the full verdict/report (informational)
    }

    // ─── Storage ───────────────────────────────────────────────────────────

    /// @notice keccak256(verdict content) => attestation record.
    mapping(bytes32 => Attestation) private _attestations;

    /// @notice Total distinct hashes notarized.
    uint256 public attestationCount;

    // ─── Events ────────────────────────────────────────────────────────────

    event AttestationCommitted(
        bytes32 indexed verdictHash,
        address indexed committer,
        RiskLevel riskLevel,
        string projectId,
        string evidenceUri,
        uint64 committedAt
    );

    // ─── Errors ────────────────────────────────────────────────────────────

    /// @notice Raised when committing the zero hash.
    error ZeroHash();

    // ─── External API ──────────────────────────────────────────────────────

    /**
     * @notice Notarize a verdict (or any evidence document) by its keccak256
     *         hash. Idempotent: re-committing an existing hash is a no-op.
     * @param verdictHash keccak256 of the full verdict/report content.
     * @param riskLevel   informational risk classification of the verdict.
     * @param projectId   informational project identifier the verdict concerns.
     * @param evidenceUri informational pointer to the full content
     *                    (e.g. https URL, IPFS CID, arweave tx).
     */
    function commit(
        bytes32 verdictHash,
        RiskLevel riskLevel,
        string calldata projectId,
        string calldata evidenceUri
    ) external {
        if (verdictHash == bytes32(0)) revert ZeroHash();
        if (_attestations[verdictHash].committer != address(0)) return; // already notarized

        _attestations[verdictHash] = Attestation({
            committer: msg.sender,
            committedAt: uint64(block.timestamp),
            riskLevel: riskLevel,
            projectId: projectId,
            evidenceUri: evidenceUri
        });

        unchecked {
            ++attestationCount;
        }

        emit AttestationCommitted(
            verdictHash,
            msg.sender,
            riskLevel,
            projectId,
            evidenceUri,
            uint64(block.timestamp)
        );
    }

    // ─── Views ─────────────────────────────────────────────────────────────

    /**
     * @notice Read the attestation recorded for a verdict hash.
     * @return exists     true when the hash has been notarized.
     * @return attestation the stored record (zeroed when !exists).
     */
    function getAttestation(bytes32 verdictHash)
        external
        view
        returns (bool exists, Attestation memory attestation)
    {
        attestation = _attestations[verdictHash];
        return (attestation.committer != address(0), attestation);
    }

    /**
     * @notice Convenience check: has this verdict hash been notarized?
     */
    function isNotarized(bytes32 verdictHash) external view returns (bool) {
        return _attestations[verdictHash].committer != address(0);
    }
}
