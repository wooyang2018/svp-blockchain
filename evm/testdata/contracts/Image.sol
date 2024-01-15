// SPDX-License-Identifier: GPL-3.0
pragma solidity >0.6.0;

contract Image {
    mapping(uint => string) image;
    uint x = 0;
    address owner;

    event ImageSave(uint i);

    constructor() {
        owner = msg.sender;
    }

    function save(string calldata s) public returns (uint _index) {
        require(msg.sender == owner);
        x++;
        image[x] = s;
        emit ImageSave(x);
        return x;
    }

    function get(uint i) public view returns (string memory _image){
        require(msg.sender == owner);
        return image[i];
    }

    function getThis() view public returns (address _addr){
        return address(this);
    }
}