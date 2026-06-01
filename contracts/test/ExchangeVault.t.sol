// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {ExchangeVault} from "../src/ExchangeVault.sol";
import {MockUSDT} from "../src/MockUSDT.sol";

contract ExchangeVaultTest is Test {
    ExchangeVault vault;
    MockUSDT usdt;
    address operator = address(0x1);
    address user = address(0x2);

    function setUp() public {
        vault = new ExchangeVault(operator);
        usdt = new MockUSDT();
        usdt.mint(user, 10_000_000);
        vm.deal(user, 10 ether);
    }

    function testDepositETH() public {
        vm.prank(user);
        vm.expectEmit(true, false, false, true);
        emit ExchangeVault.DepositETH(user, 1 ether, bytes32(uint256(1)));
        vault.depositETH{value: 1 ether}(bytes32(uint256(1)));
        assertEq(address(vault).balance, 1 ether);
    }

    function testDepositERC20() public {
        vm.startPrank(user);
        usdt.approve(address(vault), 1_000_000);
        vault.depositERC20(address(usdt), 1_000_000, bytes32(uint256(2)));
        vm.stopPrank();
        assertEq(usdt.balanceOf(address(vault)), 1_000_000);
    }

    function testWithdrawETHByOperator() public {
        vm.deal(address(vault), 2 ether);
        vm.prank(operator);
        vault.withdrawETH(user, 1 ether, bytes32(uint256(3)));
        assertEq(user.balance, 11 ether);
    }

    function testPauseBlocksDeposit() public {
        vm.prank(vault.owner());
        vault.pause();
        vm.prank(user);
        vm.expectRevert();
        vault.depositETH{value: 1 ether}(bytes32(0));
    }
}
