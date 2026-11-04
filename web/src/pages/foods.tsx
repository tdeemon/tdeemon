import { Outlet, useChildMatches } from '@tanstack/react-router'

import { FoodBrowser } from '@/components/foods/food-browser'
import { useDesktop } from '@/lib/media'

// FoodsPage is the user's foods (FoodBrowser) and the food open of them,
// or a new one: on a desktop side by side under the title, each pane
// scrolled on its own and the page not, so the food's end is in reach
// as the list's is; on a phone one or the other.
export function FoodsPage() {
  const desktop = useDesktop()
  // A food, or a new one, is open; else the foods alone.
  const open = useChildMatches({
    select: (matches) => matches.some((m) => m.routeId !== '/app/foods/'),
  })

  if (!desktop) {
    return <div className="mx-auto max-w-2xl">{open ? <Outlet /> : <FoodBrowser />}</div>
  }
  return <FoodBrowser detail={<Outlet />} />
}
