// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script} from "forge-std/Script.sol";
import {console2} from "forge-std/console2.sol";
import {MockUSDT} from "../src/MockUSDT.sol";
import {ExchangeVault} from "../src/ExchangeVault.sol";

contract DeployLocal is Script {
    function run() external {
        uint256 deployerKey = vm.envOr("PRIVATE_KEY", uint256(0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80));
        address deployer = vm.addr(deployerKey);
        address operator = vm.envOr("OPERATOR_ADDRESS", deployer);

        vm.startBroadcast(deployerKey);

        MockUSDT usdt = new MockUSDT();
        ExchangeVault vault = new ExchangeVault(operator);

        usdt.mint(deployer, 1_000_000_000_000); // 1M USDT (6 decimals)

        vm.stopBroadcast();

        string memory json = string.concat(
            '{\n  "chainId": 31337,\n  "mockUSDT": "', vm.toString(address(usdt)), '",\n  "exchangeVault": "', vm.toString(address(vault)), '",\n  "operator": "', vm.toString(operator), '"\n}'
        );
        vm.writeFile("../deployments/local.json", json);

        console2.log("MockUSDT:", address(usdt));
        console2.log("ExchangeVault:", address(vault));
        console2.log("Operator:", operator);
    }
}
