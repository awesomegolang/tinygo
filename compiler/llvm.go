package compiler

import (
	"github.com/aykevl/go-llvm"
)

// This file contains helper functions for LLVM that are not exposed in the Go
// bindings.

// Return a list of values (actually, instructions) where this value is used as
// an operand.
func getUses(value llvm.Value) []llvm.Value {
	if value.IsNil() {
		return nil
	}
	var uses []llvm.Value
	use := value.FirstUse()
	for !use.IsNil() {
		uses = append(uses, use.User())
		use = use.NextUse()
	}
	return uses
}

// splitBasicBlock splits a LLVM basic block into two parts. All instructions
// after afterInst are moved into a new basic block (created right after the
// current one) with the given name.
func (c *Compiler) splitBasicBlock(afterInst llvm.Value, insertAfter llvm.BasicBlock, name string) llvm.BasicBlock {
	newBlock := c.ctx.InsertBasicBlock(insertAfter, name)
	var nextInstructions []llvm.Value // values to move
	var phiNodes []llvm.Value         // PHI nodes to update

	// Collect to-be-moved instructions.
	inst := afterInst
	for {
		inst = llvm.NextInstruction(inst)
		if inst.IsNil() {
			break
		}
		nextInstructions = append(nextInstructions, inst)
		for _, use := range getUses(inst) {
			if !use.IsAPHINode().IsNil() {
				phiNodes = append(phiNodes, use)
			}
		}
	}

	// Move instructions.
	c.builder.SetInsertPointAtEnd(newBlock)
	for _, inst := range nextInstructions {
		inst.RemoveFromParentAsInstruction()
		c.builder.Insert(inst)
	}

	// Update PHI nodes.
	for _, phi := range phiNodes {
		c.builder.SetInsertPointBefore(phi)
		newPhi := c.builder.CreatePHI(phi.Type(), "")
		count := phi.IncomingCount()
		incomingVals := make([]llvm.Value, count)
		incomingBlocks := make([]llvm.BasicBlock, count)
		for i := 0; i < count; i++ {
			value := phi.IncomingValue(i)
			var block llvm.BasicBlock
			if value.IsConstant() {
				block = phi.IncomingBlock(i)
			} else {
				block = value.InstructionParent()
			}
			incomingVals[i] = value
			incomingBlocks[i] = block
		}
		newPhi.AddIncoming(incomingVals, incomingBlocks)
		phi.ReplaceAllUsesWith(newPhi)
		phi.EraseFromParentAsInstruction()
	}

	return newBlock
}
