// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Test} from "forge-std/Test.sol";
import {TesseraAttestations} from "../src/TesseraAttestations.sol";

contract TesseraAttestationsTest is Test {
    TesseraAttestations public registry;

    address public alice = makeAddr("alice");
    address public bob = makeAddr("bob");

    // keccak256("verdict: octant ep-23 project-x — HIGH RISK, whale 0.71, gini 0.83")
    bytes32 public constant SAMPLE_HASH =
        0xa1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90;

    function setUp() public {
        registry = new TesseraAttestations();
    }

    // ─── commit ────────────────────────────────────────────────────────────

    function test_CommitStoresAttestation() public {
        vm.prank(alice);
        registry.commit(
            SAMPLE_HASH,
            TesseraAttestations.RiskLevel.HIGH,
            "octant:ep-23:project-x",
            "https://tessera.example/verdicts/ep-23-x"
        );

        (bool exists, TesseraAttestations.Attestation memory att) =
            registry.getAttestation(SAMPLE_HASH);

        assertTrue(exists);
        assertEq(att.committer, alice);
        assertEq(att.projectId, "octant:ep-23:project-x");
        assertEq(att.evidenceUri, "https://tessera.example/verdicts/ep-23-x");
        assertEq(uint8(att.riskLevel), uint8(TesseraAttestations.RiskLevel.HIGH));
        assertEq(att.committedAt, block.timestamp);
        assertEq(registry.attestationCount(), 1);
    }

    function test_CommitEmitsEvent() public {
        vm.prank(alice);
        vm.expectEmit(true, true, false, true);
        emit TesseraAttestations.AttestationCommitted(
            SAMPLE_HASH,
            alice,
            TesseraAttestations.RiskLevel.MEDIUM,
            "gitcoin:gr20:project-y",
            "ipfs://bafy...",
            uint64(block.timestamp)
        );
        registry.commit(
            SAMPLE_HASH,
            TesseraAttestations.RiskLevel.MEDIUM,
            "gitcoin:gr20:project-y",
            "ipfs://bafy..."
        );
    }

    function test_RevertWhen_ZeroHash() public {
        vm.prank(alice);
        vm.expectRevert(TesseraAttestations.ZeroHash.selector);
        registry.commit(
            bytes32(0),
            TesseraAttestations.RiskLevel.UNKNOWN,
            "",
            ""
        );
    }

    function test_RecommitIsNoop() public {
        vm.startPrank(alice);
        registry.commit(SAMPLE_HASH, TesseraAttestations.RiskLevel.HIGH, "a", "uri-a");
        (, TesseraAttestations.Attestation memory first) = registry.getAttestation(SAMPLE_HASH);
        uint64 firstTs = first.committedAt;

        // warp forward; a re-commit must not overwrite the original record
        vm.warp(block.timestamp + 1 days);
        registry.commit(SAMPLE_HASH, TesseraAttestations.RiskLevel.LOW, "b", "uri-b");
        vm.stopPrank();

        (, TesseraAttestations.Attestation memory att) = registry.getAttestation(SAMPLE_HASH);
        assertEq(att.committedAt, firstTs);
        assertEq(att.projectId, "a"); // original data preserved
        assertEq(uint8(att.riskLevel), uint8(TesseraAttestations.RiskLevel.HIGH));
        assertEq(registry.attestationCount(), 1); // count unchanged
    }

    function test_AnyoneCanCommit() public {
        vm.prank(bob); // not an admin — there is no admin
        registry.commit(SAMPLE_HASH, TesseraAttestations.RiskLevel.LOW, "p", "uri");

        (bool exists,) = registry.getAttestation(SAMPLE_HASH);
        assertTrue(exists);
    }

    // ─── views ─────────────────────────────────────────────────────────────

    function test_UnknownHashIsNotNotarized() public view {
        assertFalse(registry.isNotarized(bytes32(uint256(0xdeadbeef))));
        (bool exists,) = registry.getAttestation(bytes32(uint256(0xdeadbeef)));
        assertFalse(exists);
    }

    function test_CountTracksDistinctHashes() public {
        vm.startPrank(alice);
        registry.commit(SAMPLE_HASH, TesseraAttestations.RiskLevel.HIGH, "a", "u");
        registry.commit(bytes32(uint256(1)), TesseraAttestations.RiskLevel.LOW, "b", "u");
        registry.commit(bytes32(uint256(2)), TesseraAttestations.RiskLevel.LOW, "c", "u");
        registry.commit(SAMPLE_HASH, TesseraAttestations.RiskLevel.LOW, "a2", "u"); // dup
        vm.stopPrank();

        assertEq(registry.attestationCount(), 3);
    }
}
