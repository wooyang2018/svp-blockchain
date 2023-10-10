// SPDX-License-Identifier: GPL-3.0
pragma solidity >0.6.0;

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

    function retrieve() public view returns (uint256){
        return number;
    }

    function close() public {
        selfdestruct(owner);
    }
}