// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./IERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

interface IToken {
    function transfer(address to, uint256 amount) external returns (bool);
}

library MathLib {
    function add(uint256 a, uint256 b) internal pure returns (uint256) {
        return a + b;
    }
}

contract Token is Ownable {
    uint256 public totalSupply;
    event Transfer(address indexed from, address indexed to, uint256 value);
    struct Account { uint256 balance; }
    enum State { Active, Paused }
    constructor(uint256 supply) { totalSupply = supply; }
    function mint(address to, uint256 amount) public onlyOwner returns (bool) {
        return true;
    }
    function _burn(uint256 amount) internal {}
}
