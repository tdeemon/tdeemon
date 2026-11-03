// The classes a food's page and a recipe's share.

// detailHeader is the page's title: on a desktop it stays at the top of
// its pane as the pane scrolls, a line under it, over what goes under
// (the pane's scroll-padding keeps a focused control clear of it).
export const detailHeader =
  'lg:sticky lg:top-0 lg:z-10 lg:-mt-4 lg:border-b lg:bg-background lg:py-4'

// detailColumns are the page's two columns, where its pane is wide
// enough (a desktop's): what the food is and does, and its parts (its
// servings, a recipe's ingredients); one column, the first above, where
// it is not. What it brings goes under them, across both, its segments
// of amounts whole.
export const detailColumns = 'grid grid-cols-1 gap-4 @2xl:grid-cols-2 @2xl:items-start'
