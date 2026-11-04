import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import { use, useEffect, useRef, type ReactNode } from 'react'

import { SettingsAvatar } from '@/components/settings-avatar'
import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { FocusTitle } from '@/lib/focus-title'
import { formPage } from '@/lib/form-page'
import { useDesktop } from '@/lib/media'

// SubPage is a page of the plan's list or the settings': the way back to
// the list, its title (titleId names it, for a form it holds) and what it
// holds. A desktop's settings have their list beside the page, under
// their name: the page has no way back, and its title is the page's
// heading (the first level, over its sections' second).
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
  const beside = useDesktop() && from === 'settings'
  const focusTitle = use(FocusTitle)
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (beside && focusTitle?.() === true) {
      heading.current?.focus()
    }
  }, [beside, focusTitle])

  return (
    <section className="max-w-2xl space-y-4">
      {!beside && (
        <div className="flex items-center justify-between gap-2">
          <Link to={from === 'plan' ? '/plan' : '/settings'} className={back}>
            <ChevronLeft aria-hidden />
            {from === 'plan' ? t.nav.plan : t.nav.settings}
          </Link>
          {/* A page of the plan's, on a phone, a tap from the settings too;
              a form, a task of its own, has only its way back. */}
          {from === 'plan' && !form && <SettingsAvatar />}
        </div>
      )}
      <header className="space-y-1">
        {/* Beside the list, a row gives it the focus (FocusTitle). */}
        <h1
          ref={heading}
          id={titleId}
          tabIndex={beside ? -1 : undefined}
          className="text-title outline-none"
        >
          {title}
        </h1>
        {hint !== undefined && <p className="text-small text-muted-foreground">{hint}</p>}
      </header>
      {children}
    </section>
  )
}
