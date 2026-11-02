import { useQueryClient } from '@tanstack/react-query'

import { $api } from './client'
import { keep } from './foods'

// A recipe shared with other users of the instance (story 403): shared by
// an address, stopped by a member's seat, and another member's
// ingredient added to one's foods. Each is a web session's.

// useShareRecipe shares a recipe of the user's: its answer, the recipe
// with its members, is kept, and the searches asked again.
export function useShareRecipe() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/recipes/{id}/shares', {
    onSuccess: (food) => keep(queryClient, food),
  })
}

// useStopSharing stops sharing a recipe with a member, or leaves it (the
// user's own seat): the recipe and the lists and searches are asked
// again, a recipe left being none of theirs to pick.
export function useStopSharing() {
  const queryClient = useQueryClient()
  return $api.useMutation('delete', '/api/v1/recipes/{id}/shares/{seat}', {
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods/{id}'] }),
        queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods/list'] }),
        queryClient.invalidateQueries({ queryKey: ['get', '/api/v1/foods'] }),
      ]),
  })
}

// useCopyIngredient adds a shared recipe's ingredient of another member's
// food to the user's foods: the new food is kept.
export function useCopyIngredient() {
  const queryClient = useQueryClient()
  return $api.useMutation('post', '/api/v1/recipes/{id}/ingredients/{position}/food', {
    onSuccess: (food) => keep(queryClient, food),
  })
}
