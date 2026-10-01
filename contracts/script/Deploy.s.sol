// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {TesseraAttestations} from "../src/TesseraAttestations.sol";

/**
 * @notice Deploy TesseraAttestations to BNB Smart Chain.
 *
 * BSC testnet (97):
 *   forge script script/Deploy.s.sol --rpc-url bsc_testnet --broadcast --verify
 *
 * The contract is immutable with no constructor args — deploy and go.
 */
contract Deploy is Script {
    function run() public returns (TesseraAttestations registry) {
        uint256 deployer = vm.envUint("DEPLOYER_PRIVATE_KEY");
        vm.startBroadcast(deployer);

        registry = new TesseraAttestations();

        vm.stopBroadcast();

        console2.logString("TesseraAttestations deployed at:");
        console2.logAddress(address(registry));
    }
}

