import { createContext } from 'react'

// FocusTitle tells the page opened beside a list (a desktop's settings)
// whether a row of the list opened it, which it says once: so the page
// gives the focus to its title then, and not when it opens of itself.
export const FocusTitle = createContext<(() => boolean) | null>(null)
