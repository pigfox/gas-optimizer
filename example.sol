pragma solidity ^0.8.0;

contract Example {
    mapping(uint => uint) public data;
    uint8 public smallNum; // Inefficient type

    function expensiveLoop(uint n) public {
        for (uint i = 0; i < n; i++) {
            uint x = data[i]; // Repeated storage read
            uint y = data[i];
        }
    }

    function redundantCalc(uint a) public pure returns (uint) {
        uint b = a * 2; // Redundant if repeated
        return b + a * 2;
    }
}