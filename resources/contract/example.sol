// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract SimpleContract {
    uint256 public intValue;
    string public stringValue;
    address public addressValue;
    
    event IntValueSet(uint256 newValue);
    event StringValueSet(string newValue);
    event AddressValueSet(address newValue);
    
    function setIntValue(uint256 _value) external {
        intValue = _value;
        emit IntValueSet(_value);
    }
    
    function getIntValue() external view returns (uint256) {
        return intValue;
    }
    
    function setStringValue(string memory _value) external {
        stringValue = _value;
        emit StringValueSet(_value);
    }
    
    function getStringValue() external view returns (string memory) {
        return stringValue;
    }
    
    function setAddressValue(address _value) external {
        addressValue = _value;
        emit AddressValueSet(_value);
    }
    
    function getAddressValue() external view returns (address) {
        return addressValue;
    }
}
