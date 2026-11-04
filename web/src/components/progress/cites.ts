import type { components } from '@/api/schema'

type Proposal = components['schemas']['ProposalBody']

// citesWeight reports whether a proposal cites the weight rather than
// the intake: one that changes the phases alone, their rate or their
// goal. Every other is of what is eaten, the TDEE's chart.
export function citesWeight(proposal: Proposal): boolean {
  const { phases, targets, training_weekdays: days, limits } = proposal.changes
  return phases.length > 0 && targets.length === 0 && days === undefined && limits.length === 0
}
