// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Callee {
    address payable private owner;
    uint256 number;

    constructor() payable {
        owner = payable(msg.sender);
    }

    function store(uint256 num) public {
        number = num;
    }

    function storedtor(uint256 num) public {
        number = num;
        selfdestruct(owner);
    }

    function retrieve() public view returns (uint256) {
        return number;
    }

    function close() public {
        selfdestruct(owner);
    }
}