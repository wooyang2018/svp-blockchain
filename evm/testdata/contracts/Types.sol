// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract DataTypes {
    uint256 public uintValue;
    string public stringValue;
    uint32 public uint32Value;
    uint16 public uint16Value;
    address public addressValue;
    bytes public bytesValue;
    bool public boolValue;
    uint256[2] public uintArray;
    uint32[] public uint32Array;

    event ValuesUpdated(
        uint256 uintValue,
        string stringValue,
        uint32 uint32Value,
        uint16 uint16Value,
        address addressValue,
        bytes bytesValue,
        bool boolValue,
        uint256[2] uintArray,
        uint32[] uint32Array
    );

    constructor(
        uint256 _uintValue,
        string memory _stringValue,
        uint32 _uint32Value,
        uint16 _uint16Value,
        address _addressValue,
        bytes memory _bytesValue,
        bool _boolValue,
        uint256[2] memory _uintArray,
        uint32[] memory _uint32Array
    ) {
        uintValue = _uintValue;
        stringValue = _stringValue;
        uint32Value = _uint32Value;
        uint16Value = _uint16Value;
        addressValue = _addressValue;
        bytesValue = _bytesValue;
        boolValue = _boolValue;
        uintArray = _uintArray;
        uint32Array = _uint32Array;

        emit ValuesUpdated(uintValue, stringValue, uint32Value, uint16Value,
            addressValue, bytesValue, boolValue, uintArray, uint32Array);
    }

    function updateValues(
        uint256 _uintValue,
        string memory _stringValue,
        uint32 _uint32Value,
        uint16 _uint16Value,
        address _addressValue,
        bytes memory _bytesValue,
        bool _boolValue,
        uint256[2] memory _uintArray,
        uint32[] memory _uint32Array
    ) public {
        uintValue = _uintValue;
        stringValue = _stringValue;
        uint32Value = _uint32Value;
        uint16Value = _uint16Value;
        addressValue = _addressValue;
        bytesValue = _bytesValue;
        boolValue = _boolValue;
        uintArray = _uintArray;
        uint32Array = _uint32Array;

        emit ValuesUpdated(uintValue, stringValue, uint32Value, uint16Value,
            addressValue, bytesValue, boolValue, uintArray, uint32Array);
    }

    function getAllValues() public view returns (
        uint256,
        string memory,
        uint32,
        uint16,
        address,
        bytes memory,
        bool,
        uint256[2] memory,
        uint32[] memory
    ){
        return (uintValue, stringValue, uint32Value, uint16Value,
            addressValue, bytesValue, boolValue, uintArray, uint32Array);
    }
}