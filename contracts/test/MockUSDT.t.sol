// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {MockUSDT} from "../src/MockUSDT.sol";

contract MockUSDTTest is Test {
    MockUSDT token;
    address user = address(0xBEEF);

    function setUp() public {
        token = new MockUSDT();
    }

    function testMintAndTransfer() public {
        token.mint(user, 1_000_000);
        assertEq(token.balanceOf(user), 1_000_000);
        assertEq(token.decimals(), 6);
    }
}
