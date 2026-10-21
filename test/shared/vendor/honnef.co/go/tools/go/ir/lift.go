// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines the lifting pass which tries to "lift" Alloc
// cells (new/local variables) into SSA registers, replacing loads
// with the dominating stored value, eliminating loads and stores, and
// inserting φ-nodes as needed.

// Cited papers and resources:
//
// Ron Cytron et al. 1991. Efficiently computing SSA form...
// https://doi.acm.org/10.1145/115372.115320
//
// Cooper, Harvey, Kennedy.  2001.  A Simple, Fast Dominance Algorithm.
// Software Practice and Experience 2001, 4:1-10.
// https://www.hipersoft.rice.edu/grads/publications/dom14.pdf
//
// Daniel Berlin, llvmdev mailing list, 2012.
// https://lists.cs.uiuc.edu/pipermail/llvmdev/2012-January/046638.html
// (Be sure to expand the whole thread.)
//
// C. Scott Ananian. 1997. The static single information form.
//
// Jeremy Singer. 2006. Static program analysis based on virtual register renaming.

// TODO(adonovan): opt: there are many optimizations worth evaluating, and
// the conventional wisdom for SSA construction is that a simple
// algorithm well engineered often beats those of better asymptotic
// complexity on all but the most egregious inputs.
//
// Danny Berlin suggests that the Cooper et al. algorithm for
// computing the dominance frontier is superior to Cytron et al.
// Furthermore he recommends that rather than computing the DF for the
// whole function then renaming all alloc cells, it may be cheaper to
// compute the DF for each alloc cell separately and throw it away.
//
// Consider exploiting liveness information to avoid creating dead
// φ-nodes which we then immediately remove.
//
// Also see many other "TODO: opt" suggestions in the code.

import (
	"fmt"
	"os"
	"slices"
)

// If true, show diagnostic information at each step of lifting.
// Very verbose.
const debugLifting = false

// domFrontier maps each block to the set of blocks in its dominance
// frontier.  The outer slice is conceptually a map keyed by
// Block.Index.  The inner slice is conceptually a set, possibly
// containing duplicates.
//
// TODO(adonovan): opt: measure impact of dups; consider a packed bit
// representation, e.g. big.Int, and bitwise parallel operations for
// the union step in the Children loop.
//
// domFrontier's methods mutate the slice's elements but not its
// length, so their receivers needn't be pointers.
type domFrontier BlockMap[[]*BasicBlock]

func (df domFrontier) add(u, v *BasicBlock) {
	df[u.Index] = append(df[u.Index], v)
}

// build builds the dominance frontier df for the dominator tree of
// fn, using the algorithm found in A Simple, Fast Dominance
// Algorithm, Figure 5.
//
// TODO(adonovan): opt: consider Berlin approach, computing pruned SSA
// by pruning the entire IDF computation, rather than merely pruning
// the DF -> IDF step.
func (df domFrontier) build(fn *Function) {
	for _, b := range fn.Blocks {
		if len(b.Preds) >= 2 {
			for _, p := range b.Preds {
				runner := p
				for runner != b.dom.idom {
					df.add(runner, b)
					runner = runner.dom.idom
				}
			}
		}
	}
}

func buildDomFrontier(fn *Function) domFrontier {
	df := make(domFrontier, len(fn.Blocks))
	df.build(fn)
	return df
}

func removeInstr(refs []Instruction, instr Instruction) []Instruction {
	s := refs
	i := -1
	for j := range s {
		if instr == s[j] {
			i = j
			break
		}
	}
	if i == -1 {
		return s
	}
	// Don't start copying elements until we find one to delete.
	for j := i + 1; j < len(s); j++ {
		if v := s[j]; v != instr {
			s[i] = v
			i++
		}
	}
	clear(s[i:]) // zero/nil out the obsolete elements, for GC
	return s[:i]
}

func clearInstrs(instrs []Instruction) {
	for i := range instrs {
		instrs[i] = nil
	}
}

func numberNodesPerBlock(f *Function) {
	for _, b := range f.Blocks {
		var base ID
		for _, instr := range b.Instrs {
			if instr == nil {
				continue
			}
			instr.setID(base)
			base++
		}
	}
}

// lift replaces local and new Allocs accessed only with
// load/store by IR registers, inserting φ-nodes where necessary.
// The result is a program in pruned SSA form.
//
// Preconditions:
// - fn has no dead blocks (blockopt has run).
// - Def/use info (Operands and Referrers) is up-to-date.
// - The dominator tree is up-to-date.
func lift(fn *Function) bool {
	// TODO(adonovan): opt: lots of little optimizations may be
	// worthwhile here, especially if they cause us to avoid
	// buildDomFrontier.  For example:
	//
	// - Alloc never loaded?  Eliminate.
	// - Alloc never stored?  Replace all loads with a zero constant.
	// - Alloc stored once?  Replace loads with dominating store;
	//   don't forget that an Alloc is itself an effective store
	//   of zero.
	// - Alloc used only within a single block?
	//   Use degenerate algorithm avoiding φ-nodes.
	// - Consider synergy with scalar replacement of aggregates (SRA).
	//   e.g. *(&x.f) where x is an Alloc.
	//   Perhaps we'd get better results if we generated this as x.f
	//   i.e. Field(x, .f) instead of Load(FieldIndex(x, .f)).
	//   Unclear.
	//
	// But we will start with the simplest correct code.
	var df domFrontier
	var newPhis BlockMap[[]newPhi]

	// During this pass we will replace some BasicBlock.Instrs
	// (allocs, loads and stores) with nil, keeping a count in
	// BasicBlock.gaps.  At the end we will reset Instrs to the
	// concatenation of all non-dead newPhis and non-nil Instrs
	// for the block, reusing the original array if space permits.

	// While we're here, we also eliminate 'rundefers'
	// instructions and ssa:deferstack() in functions that contain no
	// 'defer' instructions. Eliminate ssa:deferstack() if it does not
	// escape.
	usesDefer := false
	deferstackAlloc, deferstackCall := deferstackPreamble(fn)
	eliminateDeferStack := deferstackAlloc != nil && !deferstackAlloc.Heap

	// Determine which allocs we can lift and number them densely.
	// The renaming phase uses this numbering for compact maps.
	numAllocs := 0

	instructions := make(BlockMap[liftInstructions], len(fn.Blocks))
	for i := range instructions {
		instructions[i].insertInstructions = map[Instruction][]Instruction{}
	}

	// Number nodes, for liftable
	numberNodesPerBlock(fn)

	heads := make(BlockMap[int], len(fn.Blocks))
	for i := range heads {
		heads[i] = -1
	}
	for _, b := range fn.Blocks {
		for i, instr := range b.Instrs {
			if _, ok := instr.(*Phi); !ok {
				heads[b.Index] = i
				break
			}
		}
	}

	for _, b := range fn.Blocks {
		b.gaps = 0
		b.rundefers = 0

		for _, instr := range b.Instrs {
			switch instr := instr.(type) {
			case *Alloc:
				if !liftable(instr, instructions, heads) {
					instr.index = -1
					continue
				}

				if numAllocs == 0 {
					df = buildDomFrontier(fn)
					newPhis = make(BlockMap[[]newPhi], len(fn.Blocks))

					if debugLifting {
						title := false
						for i, blocks := range df {
							if blocks != nil {
								if !title {
									fmt.Fprintf(os.Stderr, "Dominance frontier of %s:\n", fn)
									title = true
								}
								fmt.Fprintf(os.Stderr, "\t%s: %s\n", fn.Blocks[i], blocks)
							}
						}
					}
				}
				instr.index = numAllocs
				numAllocs++
			case *Defer:
				usesDefer = true
				if eliminateDeferStack {
					// Clear _DeferStack and remove references to loads
					if instr.DeferStack != nil {
						if refs := instr.DeferStack.Referrers(); refs != nil {
							*refs = removeInstr(*refs, instr)
						}
						instr.DeferStack = nil
					}
				}
			case *RunDefers:
				b.rundefers++
			}
		}
	}

	if numAllocs > 0 {
		for _, b := range fn.Blocks {
			work := instructions[b.Index]
			for _, rename := range work.renameAllocs {
				for _, instr_ := range b.Instrs[rename.startingAt:] {
					replace(instr_, rename.from, rename.to)
				}
			}
		}

		for _, b := range fn.Blocks {
			work := instructions[b.Index]
			if len(work.insertInstructions) != 0 {
				newInstrs := make([]Instruction, 0, len(fn.Blocks)+len(work.insertInstructions)*3)
				for _, instr := range b.Instrs {
					if add, ok := work.insertInstructions[instr]; ok {
						newInstrs = append(newInstrs, add...)
					}
					newInstrs = append(newInstrs, instr)
				}
				b.Instrs = newInstrs
			}
		}

		// TODO(dh): remove inserted allocs that end up unused after lifting.

		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if instr, ok := instr.(*Alloc); ok && instr.index >= 0 {
					liftAlloc(df, instr, newPhis)
				}
			}
		}

		// renaming maps an alloc (keyed by index) to its replacement
		// value.  Initially the renaming contains nil, signifying the
		// zero constant of the appropriate type; we construct the
		// Const lazily at most once on each path through the domtree.
		// TODO(adonovan): opt: cache per-function not per subtree.
		renaming := make([]Value, numAllocs)

		// Renaming.
		rename(fn.Blocks[0], renaming, newPhis)

		simplifyPhis(newPhis)

		// Eliminate dead φ-nodes.
		markLiveNodes(fn.Blocks, newPhis)

		// Eliminate ssa:deferstack() call.
		if eliminateDeferStack {
			b := deferstackCall.block
			for i, instr := range b.Instrs {
				if instr == deferstackCall {
					b.Instrs[i] = nil
					b.gaps++
					break
				}
			}
		}
	}

	// Prepend remaining live φ-nodes to each block and possibly kill rundefers.
	for _, b := range fn.Blocks {
		var head []Instruction
		if numAllocs > 0 {
			nps := newPhis[b.Index]
			head = make([]Instruction, 0, len(nps))
			for _, np := range nps {
				if np.phi.live {
					head = append(head, np.phi)
				} else {
					for _, edge := range np.phi.Edges {
						if refs := edge.Referrers(); refs != nil {
							*refs = removeInstr(*refs, np.phi)
						}
					}
					np.phi.block = nil
				}
			}
		}

		rundefersToKill := b.rundefers
		if usesDefer {
			rundefersToKill = 0
		}

		j := len(head)
		if j+b.gaps+rundefersToKill == 0 {
			continue // fast path: no new phis or gaps
		}

		// We could do straight copies instead of element-wise copies
		// when both b.gaps and rundefersToKill are zero. However,
		// that seems to only be the case ~1% of the time, which
		// doesn't seem worth the extra branch.

		// Remove dead instructions, add phis
		ns := len(b.Instrs) + j - b.gaps - rundefersToKill
		if ns <= cap(b.Instrs) {
			// b.Instrs has enough capacity to store all instructions

			// OPT(dh): check cap vs the actually required space; if
			// there is a big enough difference, it may be worth
			// allocating a new slice, to avoid pinning memory.
			dst := b.Instrs[:cap(b.Instrs)]
			i := len(dst) - 1
			for n := len(b.Instrs) - 1; n >= 0; n-- {
				instr := dst[n]
				if instr == nil {
					continue
				}
				if !usesDefer {
					if _, ok := instr.(*RunDefers); ok {
						continue
					}
				}
				dst[i] = instr
				i--
			}
			off := i + 1 - len(head)
			// aid GC
			clearInstrs(dst[:off])
			dst = dst[off:]
			copy(dst, head)
			b.Instrs = dst
		} else {
			// not enough space, so allocate a new slice and copy
			// over.
			dst := make([]Instruction, ns)
			copy(dst, head)

			for _, instr := range b.Instrs {
				if instr == nil {
					continue
				}
				if !usesDefer {
					if _, ok := instr.(*RunDefers); ok {
						continue
					}
				}
				dst[j] = instr
				j++
			}
			b.Instrs = dst
		}
	}

	// Remove any fn.Locals that were lifted.
	j := 0
	for _, l := range fn.Locals {
		if l.index < 0 {
			fn.Locals[j] = l
			j++
		}
	}
	// Nil out fn.Locals[j:] to aid GC.
	for i := j; i < len(fn.Locals); i++ {
		fn.Locals[i] = nil
	}
	fn.Locals = fn.Locals[:j]

	return numAllocs > 0
}

func hasDirectReferrer(instr Instruction) bool {
	for _, instr := range *instr.Referrers() {
		if _, ok := instr.(*Phi); !ok {
			return true
		}
	}
	return false
}

func markLiveNodes(blocks []*BasicBlock, newPhis BlockMap[[]newPhi]) {
	// Phis may become dead due to optimization passes.

	// Phi nodes are considered live if a non-phi
	// node uses them. Once we find a node that is live, we mark all
	// of its operands as used, too.
	for _, npList := range newPhis {
		for _, np := range npList {
			phi := np.phi
			if !phi.live && hasDirectReferrer(phi) {
				markLivePhi(phi)
			}
		}
	}
	// Existing φ-nodes due to && and || operators
	// are all considered live (see Go issue 19622).
	for _, b := range blocks {
		for _, phi := range b.phis() {
			markLivePhi(phi.(*Phi))
		}
	}
}

func markLivePhi(phi *Phi) {
	phi.live = true
	for _, rand := range phi.Edges {
		if rand, ok := rand.(*Phi); ok {
			if !rand.live {
				markLivePhi(rand)
			}
		}
	}
}

// simplifyPhis replaces trivial phis with non-phi alternatives. Phi
// nodes where all edges are identical, or consist of only the phi
// itself and one other value, may be replaced with the value.
func simplifyPhis(newPhis BlockMap[[]newPhi]) {
	// find all phis that are trivial and can be replaced with a
	// non-phi value. run until we reach a fixpoint, because replacing
	// a phi may make other phis trivial.
	for changed := true; changed; {
		changed = false
		for _, npList := range newPhis {
			for _, np := range npList {
				if np.phi.live {
					// we're reusing 'live' to mean 'dead' in the context of simplifyPhis
					continue
				}
				if r, ok := isUselessPhi(np.phi); ok {
					// useless phi, replace its uses with the
					// replacement value. the dead phi pass will clean
					// up the phi afterwards.
					replaceAll(np.phi, r)
					np.phi.live = true
					changed = true
				}
			}
		}
	}
}

type BlockSet struct {
	idx    int
	values []bool
	count  int
}

func NewBlockSet(size int) *BlockSet {
	return &BlockSet{values: make([]bool, size)}
}

func (s *BlockSet) Set(s2 *BlockSet) {
	copy(s.values, s2.values)
	s.count = 0
	for _, v := range s.values {
		if v {
			s.count++
		}
	}
}

func (s *BlockSet) Num() int {
	return s.count
}

func (s *BlockSet) Has(b *BasicBlock) bool {
	if b.Index >= len(s.values) {
		return false
	}
	return s.values[b.Index]
}

// add adds b to the set and returns true if the set changed.
func (s *BlockSet) Add(b *BasicBlock) bool {
	if s.values[b.Index] {
		return false
	}
	s.count++
	s.values[b.Index] = true
	s.idx = b.Index

	return true
}

func (s *BlockSet) Clear() {
	for j := range s.values {
		s.values[j] = false
	}
	s.count = 0
}

// take removes an arbitrary element from a set s and
// returns its index, or returns -1 if empty.
func (s *BlockSet) Take() int {
	// [i, end]
	for i := s.idx; i < len(s.values); i++ {
		if s.values[i] {
			s.values[i] = false
			s.idx = i
			s.count--
			return i
		}
	}

	// [start, i)
	for i := 0; i < s.idx; i++ {
		if s.values[i] {
			s.values[i] = false
			s.idx = i
			s.count--
			return i
		}
	}

	return -1
}

// newPhi is a pair of a newly introduced φ-node and the lifted Alloc
// it replaces.
type newPhi struct {
	phi   *Phi
	alloc *Alloc
}

type liftInstructions struct {
	insertInstructions map[Instruction][]Instruction
	renameAllocs       []struct {
		from       *Alloc
		to         *Alloc
		startingAt int
	}
}

type liftableBlockDesc struct {
	// is the block (partially) unliftable, because it contains unliftable
	// instructions or is reachable by an unliftable block
	isUnliftable     bool
	hasLiftableLoad  bool
	hasLiftableOther bool
	// we need to emit stores in predecessors because the unliftable use is in
	// a phi
	storeInPreds bool

	lastLiftable    int
	firstUnliftable int
}

// liftable determines if alloc can be lifted, and records instructions to split partially liftable allocs.
//
// In the trivial case, all uses of the alloc can be lifted. This is the case when it is only used for storing into and
// loading from. In that case, no instructions are recorded.
//
// In the more complex case, the alloc is used for storing into and loading from, but it is also used as a value, for
// example because it gets passed to a function, e.g. fn(&x). In this case, uses of the alloc fall into one of two
// categories: those that can be lifted and those that can't. A boundary forms between these two categories in the
// function's control flow: Once an unliftable use is encountered, the alloc is no longer liftable for the remainder of
// the basic block the use is in, nor in any blocks reachable from it.
//
// We record instructions that split the alloc into two allocs: one that is used in liftable uses, and one that is used
// in unliftable uses. Whenever we encounter a boundary between liftable and unliftable uses or blocks, we emit a pair
// of Load and Store that copy the value from the liftable alloc into the unliftable alloc. Taking these instructions
// into account, the normal lifting machinery will completely lift the liftable alloc, store the correct lifted values
// into the unliftable alloc, and will not at all lift the unliftable alloc.
//
// In Go syntax, the transformation looks somewhat like this:
//
//	func foo() {
//		x := 32
//		if cond {
//			println(x)
//			escape(&x)
//			println(x)
//		} else {
//			println(x)
//		}
//		println(x)
//	}
//
// transforms into
//
//	func fooSplitAlloc() {
//		x := 32
//		var x_ int
//		if cond {
//			println(x)
//			x_ = x
//			escape(&x_)
//			println(x_)
//		} else {
//			println(x)
//			x_ = x
//		}
//		println(x_)
//	}
func liftable(alloc *Alloc, instructions BlockMap[liftInstructions], heads BlockMap[int]) bool {
	fn := alloc.block.parent

	// Don't lift result values in functions that defer
	// calls that may recover from panic.
	if fn.Recover != nil {
		if slices.Contains(fn.results, alloc) {
			return false
		}
	}

	blocks := fn.liftableBlockMap
	if len(blocks) != len(fn.Blocks) {
		blocks = make(BlockMap[liftableBlockDesc], len(fn.Blocks))
		fn.liftableBlockMap = blocks
	} else {
		clear(blocks)
	}
	for _, b := range fn.Blocks {
		blocks[b.Index].lastLiftable = -1
		blocks[b.Index].firstUnliftable = len(b.Instrs) + 1
	}

	// Look at all uses of the alloc and deduce which blocks have liftable or unliftable instructions.
	for _, instr := range alloc.referrers {
		// Find the first unliftable use

		desc := &blocks[instr.Block().Index]
		hasUnliftable := false
		inHead := false
		switch instr := instr.(type) {
		case *Store:
			if instr.Val == alloc {
				hasUnliftable = true
			}
		case *Load:
		case *debugRef:
		case *Phi:
			inHead = true
			hasUnliftable = true
		default:
			hasUnliftable = true
		}

		if hasUnliftable {
			desc.isUnliftable = true
			if int(instr.ID()) < desc.firstUnliftable {
				desc.firstUnliftable = int(instr.ID())
			}
			if inHead {
				desc.storeInPreds = true
				desc.firstUnliftable = 0
			}
		}
	}

	for _, instr := range alloc.referrers {
		// Find the last liftable use, taking the previously calculated firstUnliftable into consideration

		desc := &blocks[instr.Block().Index]
		if int(instr.ID()) >= desc.firstUnliftable {
			continue
		}
		hasLiftable := false
		switch instr := instr.(type) {
		case *Store:
			if instr.Val != alloc {
				desc.hasLiftableOther = true
				hasLiftable = true
			}
		case *Load:
			desc.hasLiftableLoad = true
			hasLiftable = true
		case *debugRef:
			desc.hasLiftableOther = true
		}
		if hasLiftable {
			if int(instr.ID()) > desc.lastLiftable {
				desc.lastLiftable = int(instr.ID())
			}
		}
	}

	for i := range blocks {
		// Update firstUnliftable to be one after lastLiftable. We do this to include the unliftable's preceding
		// DebugRefs in the renaming.
		if blocks[i].lastLiftable == -1 && !blocks[i].storeInPreds {
			// There are no liftable instructions (for this alloc) in this block. Set firstUnliftable to the
			// first non-head instruction to avoid inserting the store before phi instructions, which would
			// fail validation.
			first := heads[i]
			blocks[i].firstUnliftable = first
		} else {
			blocks[i].firstUnliftable = blocks[i].lastLiftable + 1
		}
	}

	// If a block is reachable by a (partially) unliftable block, then the entirety of the block is unliftable. In that
	// case, stores have to be inserted in the predecessors.
	//
	// TODO(dh): this isn't always necessary. If the block is reachable by itself, i.e. part of a loop, then if the
	// Alloc instruction is itself part of that loop, then there is a subset of instructions in the loop that can be
	// lifted. For example:
	//
	// 	for {
	// 		x := 42
	// 		println(x)
	// 		escape(&x)
	// 	}
	//
	// The x that escapes in one iteration of the loop isn't the same x that we read from on the next iteration.
	seen := make(BlockMap[bool], len(fn.Blocks))
	var dfs func(b *BasicBlock)
	dfs = func(b *BasicBlock) {
		if seen[b.Index] {
			return
		}
		seen[b.Index] = true
		desc := &blocks[b.Index]
		desc.hasLiftableLoad = false
		desc.hasLiftableOther = false
		desc.isUnliftable = true
		desc.firstUnliftable = 0
		desc.storeInPreds = true
		for _, succ := range b.Succs {
			dfs(succ)
		}
	}
	for _, b := range fn.Blocks {
		if blocks[b.Index].isUnliftable {
			for _, succ := range b.Succs {
				dfs(succ)
			}
		}
	}

	hasLiftableLoad := false
	hasLiftableOther := false
	hasUnliftable := false
	for _, b := range fn.Blocks {
		desc := &blocks[b.Index]
		hasLiftableLoad = hasLiftableLoad || desc.hasLiftableLoad
		hasLiftableOther = hasLiftableOther || desc.hasLiftableOther
		if desc.isUnliftable {
			hasUnliftable = true
		}
	}
	if !hasLiftableLoad && !hasLiftableOther {
		// There are no liftable uses
		return false
	} else if !hasUnliftable {
		// The alloc is entirely liftable without splitting
		return true
	} else if !hasLiftableLoad {
		// The alloc is not entirely liftable, and the only liftable uses are stores. While some of those stores could
		// get lifted away, it would also lead to an infinite loop when lifting to a fixpoint, because the newly created
		// allocs also get stored into repeatable and that's their only liftable uses.
		return false
	}

	// We need to insert stores for the new alloc. If a (partially) unliftable block has no unliftable
	// predecessors and the use isn't in a phi node, then the store can be inserted right before the unliftable use.
	// Otherwise, stores have to be inserted at the end of all liftable predecessors.

	newAlloc := &Alloc{Heap: true}
	newAlloc.setBlock(alloc.block)
	newAlloc.setType(alloc.typ)
	newAlloc.setSource(alloc.source)
	newAlloc.index = -1
	newAlloc.comment = "split alloc"

	{
		work := instructions[alloc.block.Index]
		work.insertInstructions[alloc] = append(work.insertInstructions[alloc], newAlloc)
	}

	predHasStore := make(BlockMap[bool], len(fn.Blocks))
	for _, b := range fn.Blocks {
		desc := &blocks[b.Index]
		bWork := &instructions[b.Index]

		if desc.isUnliftable {
			bWork.renameAllocs = append(bWork.renameAllocs, struct {
				from       *Alloc
				to         *Alloc
				startingAt int
			}{
				alloc, newAlloc, int(desc.firstUnliftable),
			})
		}

		if !desc.isUnliftable {
			continue
		}

		propagate := func(in *BasicBlock, before Instruction) {
			load := &Load{
				X: alloc,
			}
			store := &Store{
				Addr: newAlloc,
				Val:  load,
			}
			load.setType(deref(alloc.typ))
			load.setBlock(in)
			load.comment = "split alloc"
			store.setBlock(in)
			updateOperandReferrers(load)
			updateOperandReferrers(store)
			store.comment = "split alloc"

			entry := &instructions[in.Index]
			entry.insertInstructions[before] = append(entry.insertInstructions[before], load, store)
		}

		if desc.storeInPreds {
			// emit stores at the end of liftable preds
			for _, pred := range b.Preds {
				if blocks[pred.Index].isUnliftable {
					continue
				}

				if !alloc.block.Dominates(pred) {
					// Consider this cfg:
					//
					//      1
					//     /|
					//    / |
					//   ↙  ↓
					//  2--→3
					//
					// with an Alloc in block 2. It doesn't make sense to insert a store in block 1 for the jump to
					// block 3, because 1 can never see the Alloc in the first place.
					//
					// Ignoring phi nodes, an Alloc always dominates all of its uses, and phi nodes don't matter here,
					// because for the incoming edges that do matter, we do emit the stores.

					continue
				}

				if predHasStore[pred.Index] {
					// Don't generate redundant propagations. Not only is it unnecessary, it can lead to infinite loops
					// when trying to lift to a fix point, because redundant stores are liftable.
					continue
				}

				predHasStore[pred.Index] = true

				before := pred.Instrs[len(pred.Instrs)-1]
				propagate(pred, before)
			}
		} else {
			// emit store before the first unliftable use
			before := b.Instrs[desc.firstUnliftable]
			propagate(b, before)
		}
	}

	return true
}

// liftAlloc lifts alloc into registers and populates newPhis with all the φ-nodes it may require.
func liftAlloc(df domFrontier, alloc *Alloc, newPhis BlockMap[[]newPhi]) {
	fn := alloc.Parent()

	defblocks := fn.blockset(0)
	Aphi := fn.blockset(2)
	W := fn.blockset(3)

	// Compute defblocks, the set of blocks containing a
	// definition of the alloc cell.
	for _, instr := range *alloc.Referrers() {
		switch instr := instr.(type) {
		case *Store:
			defblocks.Add(instr.Block())
		}
	}
	// The Alloc itself counts as a (zero) definition of the cell.
	defblocks.Add(alloc.Block())

	if debugLifting {
		fmt.Fprintln(os.Stderr, "\tlifting ", alloc, alloc.Name())
	}

	// Φ-insertion.
	//
	// What follows is the body of the main loop of the insert-φ
	// function described by Cytron et al, but instead of using
	// counter tricks, we just reset the 'hasAlready' and 'work'
	// sets each iteration.  These are bitmaps so it's pretty cheap.

	// Initialize W and work to defblocks.

	for change := true; change; {
		change = false
		{
			// Traverse iterated dominance frontier, inserting φ-nodes.
			W.Set(defblocks)

			for i := W.Take(); i != -1; i = W.Take() {
				n := fn.Blocks[i]
				for _, y := range df[n.Index] {
					if Aphi.Add(y) {
						if len(*alloc.Referrers()) == 0 {
							continue
						}
						live := false
						for _, ref := range *alloc.Referrers() {
							if _, ok := ref.(*Load); ok {
								if y.Reaches(ref.Block()) {
									live = true
									break
								}
							}
						}
						if !live {
							continue
						}

						// Create φ-node.
						// It will be prepended to v.Instrs later, if needed.
						phi := &Phi{
							Edges: make([]Value, len(y.Preds)),
						}
						phi.comment = alloc.comment
						phi.source = alloc.source
						phi.setType(deref(alloc.Type()))
						phi.block = y
						if debugLifting {
							fmt.Fprintf(os.Stderr, "\tplace %s = %s at block %s\n", phi.Name(), phi, y)
						}
						newPhis[y.Index] = append(newPhis[y.Index], newPhi{phi, alloc})

						change = true
						if defblocks.Add(y) {
							W.Add(y)
						}
					}
				}
			}
		}
	}
}

// replaceAll replaces all intraprocedural uses of x with y,
// updating x.Referrers and y.Referrers.
// Precondition: x.Referrers() != nil, i.e. x must be local to some function.
func replaceAll(x, y Value) {
	var rands []*Value
	pxrefs := x.Referrers()
	pyrefs := y.Referrers()
	for _, instr := range *pxrefs {
		switch instr := instr.(type) {
		case *CompositeValue:
			// Special case CompositeValue because it might have very large lists of operands
			//
			// OPT(dh): this loop is still expensive for large composite values
			for i, rand := range instr.Values {
				if rand == x {
					instr.Values[i] = y
				}
			}
		default:
			rands = instr.Operands(rands[:0]) // recycle storage
			for _, rand := range rands {
				if *rand != nil {
					if *rand == x {
						*rand = y
					}
				}
			}
		}
		if pyrefs != nil {
			*pyrefs = append(*pyrefs, instr) // dups ok
		}
	}
	*pxrefs = nil // x is now unreferenced
}

func replace(instr Instruction, x, y Value) {
	args := instr.Operands(nil)
	matched := false
	for _, arg := range args {
		if *arg == x {
			*arg = y
			matched = true
		}
	}
	if matched {
		yrefs := y.Referrers()
		if yrefs != nil {
			*yrefs = append(*yrefs, instr)
		}

		xrefs := x.Referrers()
		if xrefs != nil {
			*xrefs = removeInstr(*xrefs, instr)
		}
	}
}

// renamed returns the value to which alloc is being renamed,
// constructing it lazily if it's the implicit zero initialization.
func renamed(fn *Function, renaming []Value, alloc *Alloc) Value {
	v := renaming[alloc.index]
	if v == nil {
		v = zeroConst(deref(alloc.Type()), alloc.source)
		renaming[alloc.index] = v
	}
	return v
}

// rename implements the Cytron et al-based SSA renaming algorithm, a
// preorder traversal of the dominator tree replacing all loads of
// Alloc cells with the value stored to that cell by the dominating
// store instruction.
//
// renaming is a map from *Alloc (keyed by index number) to its
// dominating stored value; newPhis[x] is the set of new φ-nodes to be
// prepended to block x.
func rename(u_ *BasicBlock, renaming_ []Value, newPhis BlockMap[[]newPhi]) {
	type worklistEntry struct {
		u        *BasicBlock
		renaming []Value
	}

	worklist := []worklistEntry{{u_, renaming_}}
	var freelist [][]Value

	for len(worklist) > 0 {
		entry := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		u, renaming := entry.u, entry.renaming

		// Each φ-node becomes the new name for its associated Alloc.
		for _, np := range newPhis[u.Index] {
			phi := np.phi
			alloc := np.alloc
			renaming[alloc.index] = phi
		}

		// Rename loads and stores of allocs.
		for i, instr := range u.Instrs {
			switch instr := instr.(type) {
			case *Alloc:
				if instr.index >= 0 { // store of zero to Alloc cell
					// Replace dominated loads by the zero value.
					renaming[instr.index] = nil
					if debugLifting {
						fmt.Fprintf(os.Stderr, "\tkill alloc %s\n", instr)
					}
					// Delete the Alloc.
					u.Instrs[i] = nil
					u.gaps++
				}

			case *Store:
				if alloc, ok := instr.Addr.(*Alloc); ok && alloc.index >= 0 { // store to Alloc cell
					// Replace dominated loads by the stored value.
					renaming[alloc.index] = instr.Val
					if debugLifting {
						fmt.Fprintf(os.Stderr, "\tkill store %s; new value: %s\n",
							instr, instr.Val.Name())
					}
					if refs := instr.Addr.Referrers(); refs != nil {
						*refs = removeInstr(*refs, instr)
					}
					if refs := instr.Val.Referrers(); refs != nil {
						*refs = removeInstr(*refs, instr)
					}
					// Delete the Store.
					u.Instrs[i] = nil
					u.gaps++
				}

			case *Load:
				if alloc, ok := instr.X.(*Alloc); ok && alloc.index >= 0 { // load of Alloc cell
					newval := renamed(u.Parent(), renaming, alloc)
					if debugLifting {
						fmt.Fprintf(os.Stderr, "\tupdate load %s = %s with %s\n",
							instr.Name(), instr, newval)
					}
					// Replace all references to the loaded value by the dominating
					// stored value.
					replaceAll(instr, newval)
					u.Instrs[i] = nil
					u.gaps++
				}

			case *debugRef:
				if x, ok := instr.X.(*Alloc); ok && x.index >= 0 {
					if instr.IsAddr {
						instr.X = renamed(u.Parent(), renaming, x)
						instr.IsAddr = false

						// Add DebugRef to instr.X's referrers.
						if refs := instr.X.Referrers(); refs != nil {
							*refs = append(*refs, instr)
						}
					} else {
						// A source expression denotes the address
						// of an Alloc that was optimized away.
						instr.X = nil

						// Delete the DebugRef.
						u.Instrs[i] = nil
						u.gaps++
					}
				}
			}
		}

		// For each φ-node in a CFG successor, rename the edge.
		for _, v := range u.Succs {
			phis := newPhis[v.Index]
			if len(phis) == 0 {
				continue
			}
			i := v.predIndex(u)
			for _, np := range phis {
				phi := np.phi
				alloc := np.alloc
				newval := renamed(u.Parent(), renaming, alloc)
				if debugLifting {
					fmt.Fprintf(os.Stderr, "\tsetphi %s edge %s -> %s (#%d) (alloc=%s) := %s\n",
						phi.Name(), u, v, i, alloc.Name(), newval.Name())
				}
				phi.Edges[i] = newval
				if prefs := newval.Referrers(); prefs != nil {
					*prefs = append(*prefs, phi)
				}
			}
		}

		// Continue depth-first recursion over domtree, pushing a
		// fresh copy of the renaming map for each subtree.
		for _, v := range slices.Backward(u.dom.children) {
			var r []Value
			if len(freelist) == 0 {
				r = make([]Value, len(renaming))
			} else {
				r = freelist[len(freelist)-1]
				freelist = freelist[:len(freelist)-1]
			}
			copy(r, renaming)
			worklist = append(worklist, worklistEntry{v, r})
		}
		freelist = append(freelist, renaming)
	}
}

func simplifyConstantCompositeValues(fn *Function) bool {
	changed := false

	for _, b := range fn.Blocks {
		n := 0
		for _, instr := range b.Instrs {
			replaced := false

			if cv, ok := instr.(*CompositeValue); ok {
				ac := &AggregateConst{}
				ac.typ = cv.typ
				replaced = true
				for _, v := range cv.Values {
					if c, ok := v.(Constant); ok {
						ac.Values = append(ac.Values, c)
					} else {
						replaced = false
						break
					}
				}
				if replaced {
					replaceAll(cv, ac)
					killInstruction(cv)
				}

			}

			if replaced {
				changed = true
			} else {
				b.Instrs[n] = instr
				n++
			}
		}

		clearInstrs(b.Instrs[n:])
		b.Instrs = b.Instrs[:n]
	}

	return changed
}

func updateOperandReferrers(instr Instruction) {
	for _, op := range instr.Operands(nil) {
		refs := (*op).Referrers()
		if refs != nil {
			*refs = append(*refs, instr)
		}
	}
}

// deferstackPreamble returns the *Alloc and ssa:deferstack() call for fn.deferstack.
func deferstackPreamble(fn *Function) (*Alloc, *Call) {
	if alloc, _ := fn.vars[fn.deferstack].(*Alloc); alloc != nil {
		for _, ref := range *alloc.Referrers() {
			if ref, _ := ref.(*Store); ref != nil && ref.Addr == alloc {
				if call, _ := ref.Val.(*Call); call != nil {
					return alloc, call
				}
			}
		}
	}
	return nil, nil
}
