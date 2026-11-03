import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import type { ReactNode } from 'react'

import { SettingsAvatar } from '@/components/settings-avatar'
import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { formPage } from '@/lib/form-page'

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
  const form = useRouterState({ select: (state) => formPage(state.location) })
  const back = buttonVariants({ variant: 'ghost', className: '-ml-3' })

  return (
    <section className="max-w-2xl space-y-4">
      <div className="flex items-center justify-between gap-2">
        <Link to={from === 'plan' ? '/plan' : '/settings'} className={back}>
          <ChevronLeft aria-hidden />
          {from === 'plan' ? t.nav.plan : t.nav.settings}
        </Link>
        {/* A page of the plan's, on a phone, a tap from the settings too;
            a form, a task of its own, has only its way back. */}
        {from === 'plan' && !form && <SettingsAvatar />}
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
