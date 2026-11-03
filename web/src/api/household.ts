import { useQueryClient, type QueryClient } from '@tanstack/react-query'

import { $api } from './client'
import type { components } from './schema'

// A household (story 408): users of the instance who keep one kitchen,
// every food and recipe of a member each member's. A web session's alone.

export type Household = components['schemas']['HouseholdBody']
export type HouseholdInvite = components['schemas']['HouseholdInviteBody']
export type MergeReport = components['schemas']['MergeReportBody']

// householdQuery is the household: its members, and the invites waiting.
export const householdQuery = $api.queryOptions('get', '/api/v1/household')

const householdKey = householdQuery.queryKey

// codeOf is the code a household's refusal says at where (body.email,
// path.id, body.fingerprint): no_user, self, deactivated, member,
// other_household, waiting, recipe_cycle or merge_changed; undefined for
// any other error.
export function codeOf(error: unknown, where: string): string | undefined {
  if (typeof error !== 'object' || error === null || !('errors' in error)) {
    return undefined
  }
  const errors: unknown[] = Array.isArray(error.errors) ? error.errors : []
  for (const e of errors) {
    if (
      typeof e === 'object' &&
      e !== null &&
      'location' in e &&
      'value' in e &&
      e.location === where &&
      typeof e.value === 'string'
    ) {
      return e.value
    }
  }
  return undefined
}

// kitchenChanged asks again the household and everything of the foods: a
// join merges the kitchens, and a leave keeps them apart from then on.
function kitchenChanged(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: householdKey }),
    queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods/{id}'] }),
    queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods/list'] }),
    queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods'] }),
  ])
}

// useInvite invites a user by their address; the household is asked again.
export function useInvite() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/household/invites', {
    onSuccess: () => queryClient.invalidateQueries({ queryKey: householdKey }),
  })
}

// useDeleteInvite revokes an invite of the household's, or declines one
// for the user's answer.
export function useDeleteInvite() {
  const queryClient = useQueryClient()
  return $api.useMutation('delete', '/api/v1/household/invites/{id}', {
    onSettled: () => queryClient.invalidateQueries({ queryKey: householdKey }),
  })
}

// useAccept joins the household of an invite: its answer is the household,
// kept, and the foods asked again.
export function useAccept() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/household/invites/{id}/accept', {
    onSuccess: async (household) => {
      queryClient.setQueryData(householdKey, household)
      await kitchenChanged(queryClient)
    },
  })
}

// useLeave leaves the user's household.
export function useLeave() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/household/leave', {
    onSuccess: () => kitchenChanged(queryClient),
  })
}
