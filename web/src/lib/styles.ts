// The classes the app's own pages and form fields share.

// narrowPage is a page's outside the app's sections (signing in, not
// found, an error before the sections have their frame): a column in
// the middle, its sides clear of a phone's notch in landscape.
export const narrowPage =
  'mx-auto max-w-3xl space-y-4 pt-6 pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))]'

// inputClass is a text field's, an input's, a textarea's or a select's:
// 44 px high, as a button, and 16 px text, below which a phone zooms the
// page into a field when it takes the focus.
export const inputClass =
  'h-11 w-full rounded-md border border-input bg-card px-3.5 text-base outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 aria-invalid:border-destructive'

// fieldLabel is a field's label: small, in sentence case, above the field.
export const fieldLabel = 'text-small text-muted-foreground'

// chipClass is a chip's, one of a few options in a row (ChipGroup's, the
// meal's lists to search): a control's edge, 32 px high to the eye and
// 44 px to a finger (an ::after over it, 6 px above and below). The one
// chosen is the brand's soft green with a check, as a chosen segment
// is: chosenChip, for a chip chosen by its radio, and activeChip, for
// a tab.
export const chipClass =
  "relative flex h-8 min-w-11 cursor-pointer items-center justify-center gap-1 rounded-sm border border-control-border bg-card px-2.5 text-small whitespace-nowrap transition-colors outline-none after:absolute after:inset-x-0 after:-inset-y-1.5 after:content-[''] motion-reduce:transition-none"
export const chosenChip =
  'has-checked:border-transparent has-checked:bg-primary-soft has-checked:font-semibold has-checked:text-primary has-focus-visible:ring-2 has-focus-visible:ring-ring has-focus-visible:ring-offset-2 has-focus-visible:ring-offset-background'
export const activeChip =
  'focus-visible:ring-2 focus-visible:ring-ring data-active:border-transparent data-active:bg-primary-soft data-active:font-semibold data-active:text-primary'

// linkTarget is a link's or a button's that looks like one, on a line of
// its own: as high as its words to the eye, 44 px or more to a finger (an
// ::after over it, 14 px above and below).
export const linkTarget =
  "relative after:absolute after:inset-x-0 after:-inset-y-3.5 after:content-['']"
