// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// A base contract.
contract Base {
    function greet() public {}
}

interface IRunner {
    function run() external;
}
