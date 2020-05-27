// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"
	"fmt"
	"github.com/AsimovNetwork/asimov/protos"
	"math/big"

	"github.com/AsimovNetwork/asimov/common"
	"github.com/AsimovNetwork/asimov/crypto"
	"github.com/AsimovNetwork/asimov/vm/fvm/core/types"
	"github.com/AsimovNetwork/asimov/vm/fvm/math"
	"github.com/AsimovNetwork/asimov/vm/fvm/params"
)

var (
	tt255                    = math.BigPow(2, 255)
	errWriteProtection       = errors.New("fvm: write protection")
	errReturnDataOutOfBounds = errors.New("fvm: return data out of bounds")
	errExecutionReverted     = errors.New("fvm: execution reverted")
	errMaxCodeSizeExceeded   = errors.New("fvm: max code size exceeded")
)

func opAdd(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	math.U256(y.Add(x, y))

	interpreter.intPool.put(x)
	return nil, nil
}

func opSub(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	math.U256(y.Sub(x, y))

	interpreter.intPool.put(x)
	return nil, nil
}

func opMul(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(math.U256(x.Mul(x, y)))

	interpreter.intPool.put(y)

	return nil, nil
}

func opDiv(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if y.Sign() != 0 {
		math.U256(y.Div(x, y))
	} else {
		y.SetUint64(0)
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opSdiv(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())
	res := interpreter.intPool.getZero()

	if y.Sign() == 0 || x.Sign() == 0 {
		stack.push(res)
	} else {
		if x.Sign() != y.Sign() {
			res.Div(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Div(x.Abs(x), y.Abs(y))
		}
		stack.push(math.U256(res))
	}
	interpreter.intPool.put(x, y)
	return nil, nil
}

func opMod(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if y.Sign() == 0 {
		stack.push(x.SetUint64(0))
	} else {
		stack.push(math.U256(x.Mod(x, y)))
	}
	interpreter.intPool.put(y)
	return nil, nil
}

func opSmod(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())
	res := interpreter.intPool.getZero()

	if y.Sign() == 0 {
		stack.push(res)
	} else {
		if x.Sign() < 0 {
			res.Mod(x.Abs(x), y.Abs(y))
			res.Neg(res)
		} else {
			res.Mod(x.Abs(x), y.Abs(y))
		}
		stack.push(math.U256(res))
	}
	interpreter.intPool.put(x, y)
	return nil, nil
}

func opExp(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	base, exponent := stack.pop(), stack.pop()
	stack.push(math.Exp(base, exponent))

	interpreter.intPool.put(base, exponent)

	return nil, nil
}

func opSignExtend(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	back := stack.pop()
	if back.Cmp(big.NewInt(31)) < 0 {
		bit := uint(back.Uint64()*8 + 7)
		num := stack.pop()
		mask := back.Lsh(common.Big1, bit)
		mask.Sub(mask, common.Big1)
		if num.Bit(int(bit)) > 0 {
			num.Or(num, mask.Not(mask))
		} else {
			num.And(num, mask)
		}

		stack.push(math.U256(num))
	}

	interpreter.intPool.put(back)
	return nil, nil
}

func opNot(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.peek()
	math.U256(x.Not(x))
	return nil, nil
}

func opLt(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) < 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opGt(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) > 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opSlt(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()

	xSign := x.Cmp(tt255)
	ySign := y.Cmp(tt255)

	switch {
	case xSign >= 0 && ySign < 0:
		y.SetUint64(1)

	case xSign < 0 && ySign >= 0:
		y.SetUint64(0)

	default:
		if x.Cmp(y) < 0 {
			y.SetUint64(1)
		} else {
			y.SetUint64(0)
		}
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opSgt(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()

	xSign := x.Cmp(tt255)
	ySign := y.Cmp(tt255)

	switch {
	case xSign >= 0 && ySign < 0:
		y.SetUint64(0)

	case xSign < 0 && ySign >= 0:
		y.SetUint64(1)

	default:
		if x.Cmp(y) > 0 {
			y.SetUint64(1)
		} else {
			y.SetUint64(0)
		}
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opEq(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	if x.Cmp(y) == 0 {
		y.SetUint64(1)
	} else {
		y.SetUint64(0)
	}
	interpreter.intPool.put(x)
	return nil, nil
}

func opIszero(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.peek()
	if x.Sign() > 0 {
		x.SetUint64(0)
	} else {
		x.SetUint64(1)
	}
	return nil, nil
}

func opAnd(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(x.And(x, y))

	interpreter.intPool.put(y)
	return nil, nil
}

func opOr(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	y.Or(x, y)

	interpreter.intPool.put(x)
	return nil, nil
}

func opXor(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.peek()
	y.Xor(x, y)

	interpreter.intPool.put(x)
	return nil, nil
}

func opByte(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	th, val := stack.pop(), stack.peek()
	if th.Cmp(common.Big32) < 0 {
		b := math.Byte(val, 32, int(th.Int64()))
		val.SetUint64(uint64(b))
	} else {
		val.SetUint64(0)
	}
	interpreter.intPool.put(th)
	return nil, nil
}

func opAddmod(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(common.Big0) > 0 {
		x.Add(x, y)
		x.Mod(x, z)
		stack.push(math.U256(x))
	} else {
		stack.push(x.SetUint64(0))
	}
	interpreter.intPool.put(y, z)
	return nil, nil
}

func opMulmod(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(common.Big0) > 0 {
		x.Mul(x, y)
		x.Mod(x, z)
		stack.push(math.U256(x))
	} else {
		stack.push(x.SetUint64(0))
	}
	interpreter.intPool.put(y, z)
	return nil, nil
}

// opSHL implements Shift Left
// The SHL instruction (shift left) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the left by arg1 number of bits.
func opSHL(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := math.U256(stack.pop()), math.U256(stack.peek())
	defer interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		value.SetUint64(0)
		return nil, nil
	}
	n := uint(shift.Uint64())
	math.U256(value.Lsh(value, n))

	return nil, nil
}

// opSHR implements Logical Shift Right
// The SHR instruction (logical shift right) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the right by arg1 number of bits with zero fill.
func opSHR(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, second operand is left in the stack; accumulate result into it, and no need to push it afterwards
	shift, value := math.U256(stack.pop()), math.U256(stack.peek())
	defer interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		value.SetUint64(0)
		return nil, nil
	}
	n := uint(shift.Uint64())
	math.U256(value.Rsh(value, n))

	return nil, nil
}

// opSAR implements Arithmetic Shift Right
// The SAR instruction (arithmetic shift right) pops 2 values from the stack, first arg1 and then arg2,
// and pushes on the stack arg2 shifted to the right by arg1 number of bits with sign extension.
func opSAR(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Note, S256 returns (potentially) a new bigint, so we're popping, not peeking this one
	shift, value := math.U256(stack.pop()), math.S256(stack.pop())
	defer interpreter.intPool.put(shift) // First operand back into the pool

	if shift.Cmp(common.Big256) >= 0 {
		if value.Sign() > 0 {
			value.SetUint64(0)
		} else {
			value.SetInt64(-1)
		}
		stack.push(math.U256(value))
		return nil, nil
	}
	n := uint(shift.Uint64())
	value.Rsh(value, n)
	stack.push(math.U256(value))

	return nil, nil
}

func opSha3(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	data := memory.Get(offset.Int64(), size.Int64())
	hash := crypto.Keccak256(data)
	fvm := interpreter.fvm

	if fvm.vmConfig.EnablePreimageRecording {
		fvm.StateDB.AddPreimage(common.BytesToHash(hash), data)
	}
	stack.push(interpreter.intPool.get().SetBytes(hash))

	interpreter.intPool.put(offset, size)
	return nil, nil
}

func opAddress(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Address().Big())
	return nil, nil
}

func opBalance(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	slot := stack.peek()
	slot.Set(interpreter.fvm.StateDB.GetBalance(common.BigToAddress(slot)))
	return nil, nil
}

func opOrigin(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.fvm.Origin.Big())
	return nil, nil
}

func opCaller(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Caller().Big())
	return nil, nil
}

func opCallValue(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().Set(contract.value))
	return nil, nil
}

func opCallDataLoad(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetBytes(getDataBig(contract.Input, stack.pop(), big32)))
	return nil, nil
}

func opCallDataSize(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetInt64(int64(len(contract.Input))))
	return nil, nil
}

func opCallDataCopy(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		dataOffset = stack.pop()
		length     = stack.pop()
	)
	memory.Set(memOffset.Uint64(), length.Uint64(), getDataBig(contract.Input, dataOffset, length))

	interpreter.intPool.put(memOffset, dataOffset, length)
	return nil, nil
}

func opReturnDataSize(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetUint64(uint64(len(interpreter.returnData))))
	return nil, nil
}

func opReturnDataCopy(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		dataOffset = stack.pop()
		length     = stack.pop()

		end = interpreter.intPool.get().Add(dataOffset, length)
	)
	defer interpreter.intPool.put(memOffset, dataOffset, length, end)

	if end.BitLen() > 64 || uint64(len(interpreter.returnData)) < end.Uint64() {
		return nil, errReturnDataOutOfBounds
	}
	memory.Set(memOffset.Uint64(), length.Uint64(), interpreter.returnData[dataOffset.Uint64():end.Uint64()])

	return nil, nil
}

func opExtCodeSize(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	slot := stack.peek()
	if IsSystemContract(slot) {
		slot.SetUint64(uint64(1))
		return nil, nil
	}
	slot.SetUint64(uint64(interpreter.fvm.StateDB.GetCodeSize(common.BigToAddress(slot))))

	return nil, nil
}

func opCodeSize(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	l := interpreter.intPool.get().SetInt64(int64(len(contract.Code)))
	stack.push(l)

	return nil, nil
}

func opCodeCopy(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	codeCopy := getDataBig(contract.Code, codeOffset, length)
	memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

func opExtCodeCopy(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		addr       = common.BigToAddress(stack.pop())
		memOffset  = stack.pop()
		codeOffset = stack.pop()
		length     = stack.pop()
	)
	codeCopy := getDataBig(interpreter.fvm.StateDB.GetCode(addr), codeOffset, length)
	memory.Set(memOffset.Uint64(), length.Uint64(), codeCopy)

	interpreter.intPool.put(memOffset, codeOffset, length)
	return nil, nil
}

// opExtCodeHash returns the code hash of a specified account.
// There are several cases when the function is called, while we can relay everything
// to `state.GetCodeHash` function to ensure the correctness.
//   (1) Caller tries to get the code hash of a normal contract account, state
// should return the relative code hash and set it as the result.
//
//   (2) Caller tries to get the code hash of a non-existent account, state should
// return common.Hash{} and zero will be set as the result.
//
//   (3) Caller tries to get the code hash for an account without contract code,
// state should return emptyCodeHash(0xc5d246...) as the result.
//
//   (4) Caller tries to get the code hash of a precompiled account, the result
// should be zero or emptyCodeHash.
//
// It is worth noting that in order to avoid unnecessary create and clean,
// all precompile accounts on mainnet have been transferred 1 wei, so the return
// here should be emptyCodeHash.
// If the precompile account is not transferred any amount on a private or
// customized chain, the return value will be zero.
//
//   (5) Caller tries to get the code hash for an account which is marked as suicided
// in the current transaction, the code hash of this account should be returned.
//
//   (6) Caller tries to get the code hash for an account which is marked as deleted,
// this account should be regarded as a non-existent account and zero should be returned.
func opExtCodeHash(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	slot := stack.peek()
	slot.SetBytes(interpreter.fvm.StateDB.GetCodeHash(common.BigToAddress(slot)).Bytes())
	return nil, nil
}

func opGasprice(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().Set(interpreter.fvm.GasPrice))
	return nil, nil
}

func opBlockhash(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	num := stack.pop()

	n := interpreter.intPool.get().Sub(interpreter.fvm.BlockNumber, common.Big257)
	if num.Cmp(n) > 0 && num.Cmp(interpreter.fvm.BlockNumber) < 0 {
		stack.push(interpreter.fvm.GetHash(num.Uint64()).Big())
	} else {
		stack.push(interpreter.intPool.getZero())
	}
	interpreter.intPool.put(num, n)
	return nil, nil
}

func opCoinbase(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.fvm.Coinbase.Big())
	return nil, nil
}

func opTimestamp(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(interpreter.intPool.get().Set(interpreter.fvm.Time)))
	return nil, nil
}

func opNumber(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(interpreter.intPool.get().Set(interpreter.fvm.BlockNumber)))
	return nil, nil
}

func opDifficulty(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(interpreter.intPool.get().Set(interpreter.fvm.Difficulty)))
	return nil, nil
}

func opGasLimit(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(interpreter.intPool.get().SetUint64(interpreter.fvm.GasLimit)))
	return nil, nil
}

func opPop(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	interpreter.intPool.put(stack.pop())
	return nil, nil
}

func opMload(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset := stack.pop()
	val := interpreter.intPool.get().SetBytes(memory.Get(offset.Int64(), 32))
	stack.push(val)

	interpreter.intPool.put(offset)
	return nil, nil
}

func opMstore(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// pop value of the stack
	mStart, val := stack.pop(), stack.pop()
	memory.Set32(mStart.Uint64(), val)

	interpreter.intPool.put(mStart, val)
	return nil, nil
}

func opMstore8(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	off, val := stack.pop().Int64(), stack.pop().Int64()
	memory.store[off] = byte(val & 0xff)

	return nil, nil
}

func opSload(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := stack.peek()
	val := interpreter.fvm.StateDB.GetState(contract.Address(), common.BigToHash(loc))
	loc.SetBytes(val.Bytes())
	return nil, nil
}

func opSstore(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := common.BigToHash(stack.pop())
	val := stack.pop()
	interpreter.fvm.StateDB.SetState(contract.Address(), loc, common.BigToHash(val))

	interpreter.intPool.put(val)
	return nil, nil
}

func opJump(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos := stack.pop()
	if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
		nop := contract.GetOp(pos.Uint64())
		return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
	}
	*pc = pos.Uint64()

	interpreter.intPool.put(pos)
	return nil, nil
}

func opJumpi(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos, cond := stack.pop(), stack.pop()
	if cond.Sign() != 0 {
		if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
			nop := contract.GetOp(pos.Uint64())
			return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
		}
		*pc = pos.Uint64()
	} else {
		*pc++
	}

	interpreter.intPool.put(pos, cond)
	return nil, nil
}

func opJumpdest(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opPc(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetUint64(*pc))
	return nil, nil
}

func opMsize(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetInt64(int64(memory.Len())))
	return nil, nil
}

func opGas(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(interpreter.intPool.get().SetUint64(contract.Gas))
	return nil, nil
}

func opCreate(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		value        = stack.pop()
		offset, size = stack.pop(), stack.pop()
		input        = memory.Get(offset.Int64(), size.Int64())
		gas          = contract.Gas
	)
	// Apply EIP150
	gas -= gas / 64

	contract.UseGas(gas)
	res, addr, returnGas, _, suberr := interpreter.fvm.Create(contract, input, gas, value, nil, nil, nil, true)

	// Push item on the stack based on the returned error. If the ruleset is
	// homestead we must check for CodeStoreOutOfGasError (homestead only
	// rule) and treat as an error, if the ruleset is frontier we must
	// ignore this error and pretend the operation was successful.
	if suberr == ErrCodeStoreOutOfGas || suberr != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(addr.Big())
	}
	contract.Gas += returnGas
	interpreter.intPool.put(value, offset, size)

	if suberr == errExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCreate2(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		endowment    = stack.pop()
		offset, size = stack.pop(), stack.pop()
		salt         = stack.pop()
		input        = memory.Get(offset.Int64(), size.Int64())
		gas          = contract.Gas
	)

	// Apply EIP150
	gas -= gas / 64
	contract.UseGas(gas)
	res, addr, returnGas, _, suberr := interpreter.fvm.Create2(contract, input, gas, endowment, salt, nil, true)
	// Push item on the stack based on the returned error.
	if suberr != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(addr.Big())
	}
	contract.Gas += returnGas
	interpreter.intPool.put(endowment, offset, size, salt)

	if suberr == errExecutionReverted {
		return res, nil
	}
	return nil, nil
}

func opCall(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas in in interpreter.fvm.callGasTemp.
	interpreter.intPool.put(stack.pop())
	gas := interpreter.fvm.callGasTemp
	// Pop other call parameters.
	addr, value, asset, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	value = math.U256(value)
	asset = math.U256(asset)
	// Get the arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}

	assets := protos.AssetFromInt(asset)
	ret, returnGas, _, err := interpreter.fvm.Call(contract, toAddr, args, gas, value, assets, true)
	if err != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	interpreter.intPool.put(addr, value, asset, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opCallCode(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in interpreter.fvm.callGasTemp.
	interpreter.intPool.put(stack.pop())
	gas := interpreter.fvm.callGasTemp
	// Pop other call parameters.
	addr, value, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	value = math.U256(value)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}
	ret, returnGas, err := interpreter.fvm.CallCode(contract, toAddr, args, gas, value, nil)
	if err != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opDelegateCall(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in interpreter.fvm.callGasTemp.
	interpreter.intPool.put(stack.pop())
	gas := interpreter.fvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := interpreter.fvm.DelegateCall(contract, toAddr, args, gas)
	if err != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opStaticCall(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// Pop gas. The actual gas is in interpreter.fvm.callGasTemp.
	interpreter.intPool.put(stack.pop())
	gas := interpreter.fvm.callGasTemp
	// Pop other call parameters.
	addr, inOffset, inSize, retOffset, retSize := stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()
	toAddr := common.BigToAddress(addr)
	// Get arguments from the memory.
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := interpreter.fvm.StaticCall(contract, toAddr, args, gas)
	if err != nil {
		stack.push(interpreter.intPool.getZero())
	} else {
		stack.push(interpreter.intPool.get().SetUint64(1))
	}
	if err == nil || err == errExecutionReverted {
		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	interpreter.intPool.put(addr, inOffset, inSize, retOffset, retSize)
	return ret, nil
}

func opReturn(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	ret := memory.GetPtr(offset.Int64(), size.Int64())

	interpreter.intPool.put(offset, size)
	return ret, nil
}

func opRevert(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	ret := memory.GetPtr(offset.Int64(), size.Int64())

	interpreter.intPool.put(offset, size)
	return ret, nil
}

func opStop(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opSuicide(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	//balance := interpreter.fvm.StateDB.GetBalance(contract.Address())
	//interpreter.fvm.StateDB.AddBalance(common.BigToAddress(stack.pop()), balance)

	contractAddress := contract.Address()
	beneficiary := common.BigToAddress(stack.pop())
	_, err := interpreter.fvm.CalculateBalance(interpreter.fvm.View, interpreter.fvm.Block, contractAddress, nil, 0)
	if err != nil {
		return nil, errors.New("failed to calculate balance")
	}
	balanceAssets := interpreter.fvm.Block.GetBalance(contractAddress)
	allVtx := interpreter.fvm.Vtx.GetAllTransfers()

	// Copy balance and merge erc20
	netAssets := make(map[protos.Asset][]int64)
	if balanceAssets != nil {
		for _, entry := range *balanceAssets {
            assets := *entry.Asset()
            assetsbalance, ok := netAssets[assets]
            if ok {
            	if assets.IsIndivisible() {
					assetsbalance = append(assetsbalance, entry.Amount())
				} else {
					assetsbalance[0] += entry.Amount()
				}
			} else {
				assetsbalance = []int64{entry.Amount()}
			}
			netAssets[assets] = assetsbalance
		}
	}
	// merge vtx
	for assets, item := range allVtx {
		assetsbalance, ok := netAssets[assets]
		if ok {
			if assets.IsIndivisible() {
				if assetsIds, ok := item.Erc721Change[contractAddress]; ok {
					for assetsId, _ := range assetsIds {
						if assetsId > 0 {
							assetsbalance = append(assetsbalance, assetsId)
						} else {
							for i, v := range assetsbalance {
								if v == -assetsId {
									assetsbalance = append(assetsbalance[:i], assetsbalance[i+1:]...)
									break
								}
							}
						}
					}
				}
			} else {
				if amount, ok := item.Erc20Change[contractAddress]; ok {
					if assetsbalance[0] + amount < 0 {
						return nil, errors.New("failed to destruct construct, balance logic error negative amount total")
					}
					assetsbalance[0] += amount
				}
			}
		} else {
			if assets.IsIndivisible() {
				if assetsIds, ok := item.Erc721Change[contractAddress]; ok {
					for assetsId, _ := range assetsIds {
						if assetsId > 0 {
							assetsbalance = append(assetsbalance, assetsId)
						} else {
							return nil, errors.New("failed to destruct construct, balance logic error")
						}
					}
				}
			} else {
				if amount, ok := item.Erc20Change[contractAddress]; ok {
					if amount < 0 {
						return nil, errors.New("failed to destruct construct, balance logic error negative amount")
					}
					assetsbalance = []int64{amount}
				}
			}
		}
		netAssets[assets] = assetsbalance
	}

	// move asset.
	for assets, assetsbalance := range netAssets {
		assetsCopy := assets
		if assets.IsIndivisible() {
			for _, id := range assetsbalance {
				interpreter.fvm.Vtx.AppendVTransfer(contractAddress, beneficiary, big.NewInt(id), &assetsCopy)
			}
		} else {
			amount := assetsbalance[0]
			if amount > 0 {
				interpreter.fvm.Vtx.AppendVTransfer(contractAddress, beneficiary, big.NewInt(amount), &assetsCopy)
			}
		}
	}

	interpreter.fvm.StateDB.Suicide(contractAddress)
	return nil, nil
}

// following functions are used by the instruction jump  table

// make log instruction function
func makeLog(size int) executionFunc {
	return func(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		topics := make([]common.Hash, size)
		mStart, mSize := stack.pop(), stack.pop()
		for i := 0; i < size; i++ {
			topics[i] = common.BigToHash(stack.pop())
		}

		d := memory.Get(mStart.Int64(), mSize.Int64())
		interpreter.fvm.StateDB.AddLog(&types.Log{
			Address: contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block number.
			BlockNumber: interpreter.fvm.BlockNumber.Uint64(),
		})

		interpreter.intPool.put(mStart, mSize)
		return nil, nil
	}
}

// make push instruction function
func makePush(size uint64, pushByteSize int) executionFunc {
	return func(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		codeLen := len(contract.Code)

		startMin := codeLen
		if int(*pc+1) < startMin {
			startMin = int(*pc + 1)
		}

		endMin := codeLen
		if startMin+pushByteSize < endMin {
			endMin = startMin + pushByteSize
		}

		integer := interpreter.intPool.get()
		stack.push(integer.SetBytes(common.RightPadBytes(contract.Code[startMin:endMin], pushByteSize)))

		*pc += size
		return nil, nil
	}
}

// make dup instruction function
func makeDup(size int64) executionFunc {
	return func(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.dup(interpreter.intPool, int(size))
		return nil, nil
	}
}

// make swap instruction function
func makeSwap(size int64) executionFunc {
	// switch n + 1 otherwise n would be swapped with n
	size++
	return func(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.swap(int(size))
		return nil, nil
	}
}

func opFlowAssets(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	if contract.Asset() != nil {
		stack.push(new(big.Int).SetBytes(contract.Asset().Bytes()))
	} else {
		stack.push(new(big.Int))
	}
	return nil, nil
}

func opFlowCreateAsset(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	amount, coinIndex, assetType := stack.pop(), stack.pop(), stack.pop()
	if coinIndex.Cmp(common.BigMaxuint32) > 0 || coinIndex.Cmp(common.Big0) < 0 {
		return nil, errors.New("create asset coinIndex out of bounds [0, 2^32)")
	}
	if assetType.Cmp(common.Big1) > 0 || assetType.Cmp(common.Big0) < 0 {
		return nil, errors.New("create asset assetType out of bounds [0, 1]")
	}
	assetTypeU32 := uint32(assetType.Uint64())
	if assetTypeU32 & protos.InDivisibleAsset == protos.InDivisibleAsset {
		if amount.Cmp(common.BigMaxint64) > 0 || amount.Cmp(common.Big0) <= 0 {
			return nil, errors.New("create indivisible asset, amount out of bounds (0, 2^63)")
		}
	} else {
		if amount.Cmp(common.BigMaxxing) > 0 || amount.Cmp(common.Big0) < 0 {
			return nil, errors.New("create divisible asset, amount out of bounds [0, bigMaxxing 1e18]")
		}
	}
	coinIndexU32 := uint32(coinIndex.Uint64())

	err := FlowCreateAsset(amount, contract, interpreter.fvm, assetTypeU32, coinIndexU32)
	interpreter.intPool.put(amount, coinIndex, assetType)
	return nil, err
}

func opFlowMintAsset(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	amount, coinIndex := stack.pop(), stack.pop()
	if coinIndex.Cmp(common.BigMaxuint32) > 0 || coinIndex.Cmp(common.Big0) < 0 {
		return nil, errors.New("mint asset coinIndex out of bounds [0, 2^32)")
	}
	coinIndexU32 := uint32(coinIndex.Uint64())

	err := FlowMintAsset(amount, contract, interpreter.fvm, coinIndexU32)
	interpreter.intPool.put(amount, coinIndex)
	return nil, err
}

func opFlowDeployContract(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	argsOffset, templateNameOffset, category := stack.pop(), stack.pop(), stack.pop()
	category = math.U256(category)

	// category type
	categoryU16 := uint16(category.Uint64())

	// constructor
	argsSize := memory.Get(argsOffset.Int64(), 32)
	argsSizeHex := common.Bytes2Hex(argsSize)
	argsSizeU256, ok := math.ParseBig256("0x" + argsSizeHex)
	if !ok {
		return nil, errors.New("invalid constructor's param")
	}
	args := memory.Get(argsOffset.Int64()+32, argsSizeU256.Int64())

	// template name
	templateNameSize := memory.Get(templateNameOffset.Int64(), 32)
	templateNameHex := common.Bytes2Hex(templateNameSize)
	templateNameSizeU256, ok := math.ParseBig256("0x" + templateNameHex)
	templateName := string(memory.Get(templateNameOffset.Int64()+32, templateNameSizeU256.Int64()))

	contractAddr, err := FlowDeployContract(contract, interpreter.fvm, categoryU16, templateName, args)
	if err != nil {
		return nil, err
	}

	stack.push(contractAddr.Big())

	interpreter.intPool.put(argsOffset, templateNameOffset, category)

	return nil, nil
}

func opRound(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(interpreter.intPool.get().Set(interpreter.fvm.Round)))
	return nil, nil
}

func opFlowBalance(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	assetsBig, addressBig := stack.pop(), stack.pop()
	assetsBig = math.U256(assetsBig)
    address := common.BigToAddress(addressBig)
	assets := protos.AssetFromInt(assetsBig)

	balance, err := interpreter.fvm.CalculateBalance(interpreter.fvm.View, interpreter.fvm.Block, address, assets, 0)
	if err != nil {
		stack.push(new(big.Int).SetInt64(0))
		fmt.Println("GetBalance", address.String(), assets.String(), 0)
	} else {
		bigBalance := new(big.Int).SetInt64(balance)
		income := interpreter.fvm.Vtx.GetIncoming(address, assets, 0)
		bigBalance.Add(bigBalance, income)
		stack.push(bigBalance)
		fmt.Println("GetBalance", address.String(), assets.String(), bigBalance.Int64())
	}

	interpreter.intPool.put(assetsBig, addressBig)
	return nil, nil
}

func opVoteValue(pc *uint64, interpreter *FVMInterpreter, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	if interpreter.fvm.VoteValue != nil {
		voteValue := interpreter.fvm.VoteValue()
		stack.push(new(big.Int).SetInt64(voteValue))
	} else {
		stack.push(new(big.Int).SetInt64(0))
	}
	return nil, nil
}

