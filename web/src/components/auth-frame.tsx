import type { ReactNode } from 'react'

import { BrandMark } from '@/components/brand-mark'
import { useMessages } from '@/i18n/messages'

// AuthFrame is the page of a person not signed in yet (signing in, an
// invite's account): in the middle of the screen, the product's mark and
// name over what it is (tagline), and under them a card of the page's
// title and its form, its sides clear of a phone's notch in landscape.
export function AuthFrame({
  title,
  tagline,
  children,
}: {
  title: string
  tagline?: string
  children: ReactNode
}) {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center py-10 pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))]">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center gap-2 text-center">
          <p className="flex flex-col items-center gap-3 text-title">
            <BrandMark className="size-12" />
            tdeemon
          </p>
          {tagline !== undefined && <p className="text-balance text-muted-foreground">{tagline}</p>}
        </div>
        <div className="space-y-5 rounded-lg border bg-card p-5 shadow-card sm:p-6">
          <h1 className="text-h2">{title}</h1>
          {children}
        </div>
      </div>
    </main>
  )
}

// Or is the word between two ways to sign in, on a rule across.
export function Or() {
  const t = useMessages()

  return (
    <p className="flex items-center gap-3 text-small text-muted-foreground before:h-px before:flex-1 before:bg-border before:content-[''] after:h-px after:flex-1 after:bg-border after:content-['']">
      {t.signIn.or}
    </p>
  )
}
