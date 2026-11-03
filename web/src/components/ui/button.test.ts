import { expect, test } from 'vitest'

import { buttonVariants } from '@/components/ui/button'

test("a variant's border and colour win over the base's, an outline's a control's edge", () => {
  const outline = buttonVariants({ variant: 'outline' }).split(' ')
  expect(outline).toContain('border-control-border')
  expect(outline).not.toContain('border-transparent')
  expect(buttonVariants({ variant: 'outline', className: 'w-full' })).toContain('w-full')
})
