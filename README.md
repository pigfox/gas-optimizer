# gas-optimizer

A Solidity gas optimizer that analyzes Solidity code for inefficiencies and suggests improvements to reduce gas costs.

## Usage

Run the optimizer using the following command:

```sh
go run *.go example.sol

Example Reports
Report 1
Issue: Variable data[i] read multiple times in a loop.

Suggestion: Cache data[i] in memory before the loop.

Gas Savings: 797

Location: 183:125:0

Report 2
Issue: Inefficient type uint8 used for variable smallNum.

Suggestion: Use uint256 to avoid packing overhead unless tightly packed in a struct.

Gas Savings: 200

Location: 87:21:0

Report 3
Issue: Expression a * 2 computed multiple times.

Suggestion: Cache the result in a local variable.

Gas Savings: 100

Location: 320:140:0

Contributing
Feel free to submit issues or pull requests to improve the optimizer.