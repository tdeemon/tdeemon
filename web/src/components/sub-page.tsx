import { Link } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import type { ReactNode } from 'react'

import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'

// SubPage is a page of the plan's list or the settings': the way back to
// the list, its title (titleId names it, for a form it holds) and what it
// holds.
export function SubPage({
  title,
  titleId,
  from,
  hint,
  children,
}: {
  title: string
  titleId?: string
  from: 'plan' | 'settings'
  hint?: string
  children: ReactNode
}) {
  const t = useMessages()
  const back = buttonVariants({ variant: 'ghost', className: '-ml-3' })

  return (
    <section className="max-w-2xl space-y-4">
      <div>
        <Link to={from === 'plan' ? '/plan' : '/settings'} className={back}>
          <ChevronLeft aria-hidden />
          {from === 'plan' ? t.nav.plan : t.nav.settings}
        </Link>
      </div>
      <header className="space-y-1">
        <h1 id={titleId} className="text-title">
          {title}
        </h1>
        {hint !== undefined && <p className="text-small text-muted-foreground">{hint}</p>}
      </header>
      {children}
    </section>
  )
}
