// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/// @title ExchangeVault - receives ETH/ERC20 deposits and allows operator withdrawals
contract ExchangeVault is Ownable, Pausable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    address public operator;

    event DepositETH(address indexed user, uint256 amount, bytes32 indexed depositRef);
    event DepositERC20(address indexed token, address indexed user, uint256 amount, bytes32 indexed depositRef);
    event WithdrawETH(address indexed to, uint256 amount, bytes32 indexed withdrawalId);
    event WithdrawERC20(address indexed token, address indexed to, uint256 amount, bytes32 indexed withdrawalId);
    event OperatorUpdated(address indexed oldOperator, address indexed newOperator);

    error Unauthorized();
    error ZeroAddress();

    modifier onlyOperator() {
        if (msg.sender != operator && msg.sender != owner()) revert Unauthorized();
        _;
    }

    constructor(address initialOperator) Ownable(msg.sender) {
        if (initialOperator == address(0)) revert ZeroAddress();
        operator = initialOperator;
    }

    receive() external payable whenNotPaused {
        emit DepositETH(msg.sender, msg.value, bytes32(0));
    }

    /// @notice Deposit ETH with an optional deposit reference for backend tracking
    function depositETH(bytes32 depositRef) external payable whenNotPaused {
        emit DepositETH(msg.sender, msg.value, depositRef);
    }

    /// @notice Deposit ERC20 tokens (requires prior approval)
    function depositERC20(address token, uint256 amount, bytes32 depositRef) external whenNotPaused nonReentrant {
        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);
        emit DepositERC20(token, msg.sender, amount, depositRef);
    }

    /// @notice Operator withdraws ETH to user address
    function withdrawETH(address to, uint256 amount, bytes32 withdrawalId) external onlyOperator whenNotPaused nonReentrant {
        if (to == address(0)) revert ZeroAddress();
        // Checks-Effects-Interactions: emit before external transfer
        emit WithdrawETH(to, amount, withdrawalId);
        (bool ok, ) = to.call{value: amount}("");
        require(ok, "ETH transfer failed");
    }

    /// @notice Operator withdraws ERC20 to user address
    function withdrawERC20(address token, address to, uint256 amount, bytes32 withdrawalId) external onlyOperator whenNotPaused nonReentrant {
        if (to == address(0)) revert ZeroAddress();
        emit WithdrawERC20(token, to, amount, withdrawalId);
        IERC20(token).safeTransfer(to, amount);
    }

    function setOperator(address newOperator) external onlyOwner {
        if (newOperator == address(0)) revert ZeroAddress();
        emit OperatorUpdated(operator, newOperator);
        operator = newOperator;
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }
}
