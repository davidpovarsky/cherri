# Explicit numeric coercion control

Native run 44386191992 at 42e7d1c observed equality and inequality both true for the GetVariable-forwarded require zero and its mutable binding. Direct number(0) and nonzero controls behaved as expected. The independent audit preserves this failure and 20 published hashes.

This separate synthetic fixture changes only the four forwarded/bound comparisons to explicit .number operands, using the existing WFNumberContentItem coercion. The original failing fixture is byte-identical. Both operators use the same producer; direct number and nonzero observations remain independent controls. No compiler emitter, comparison policy, expected original result or native registry is changed. Local graph checks establish the controlled comparison shape and reject causal corruptions. Only a new signed Apple run can establish runtime behavior.
