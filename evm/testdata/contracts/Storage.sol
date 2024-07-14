// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Storage {
    event Transferred(address indexed from, address indexed to, uint256 amount);

    address payable private owner;
    uint256 number;

    constructor() payable {
        owner = payable(msg.sender);
    }

    function store(uint256 num) public {
        number = num;
    }

    function retrieve() public view returns (uint256){
        return number;
    }

    function withdraw(uint256 amount) public {
        require(msg.sender == owner, "Only the owner can withdraw");
        require(address(this).balance >= amount, "Insufficient balance");
        emit Transferred(address(this), msg.sender, amount);
        payable(msg.sender).transfer(amount);
    }

    function close() public {
        selfdestruct(owner);
    }
}