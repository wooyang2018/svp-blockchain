// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Callee {
    function store(uint256 num) public {}

    function retrieve() public view returns (uint256) {}

    function storedtor(uint256 num) public {}
}

contract Caller {
    Callee st;

    constructor(address _t) payable {
        st = Callee(_t);
    }

    function getNum() public view returns (uint256 result) {
        return st.retrieve();
    }

    function setA(uint256 _val) public payable returns (uint256 result) {
        st.storedtor(_val);
        return _val;
    }
}
