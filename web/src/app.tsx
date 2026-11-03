import { CSPProvider } from '@base-ui/react/csp-provider'
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { useEffect } from 'react'

import { useLocale } from '@/i18n/locale'
import type { AppRouter } from '@/router'

export function App({ router, queryClient }: { router: AppRouter; queryClient: QueryClient }) {
  const locale = useLocale()
  // The document's language follows the app's, for screen readers,
  // hyphenation and the browser's own controls.
  useEffect(() => {
    document.documentElement.lang = locale
  }, [locale])

  // Base UI renders no <style> of its own, which the server's CSP
  // refuses: index.css has its rules (base-ui-disable-scrollbar).
  return (
    <CSPProvider disableStyleElements>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </CSPProvider>
  )
}
