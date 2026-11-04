import { Drawer } from '@base-ui/react/drawer'
import { X } from 'lucide-react'
import { createContext, useContext, useRef, useState, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'

import { buttonVariants } from '@/components/ui/button'
import { useMessages } from '@/i18n/messages'
import { touchScreen, useDesktop } from '@/lib/media'
import { cn } from '@/lib/utils'

// Sheet is a panel over the page, titled: from the bottom on a phone, at
// the right on a desktop, sliding in on the sheet's curve (with less
// motion asked for, it only fades). It is dragged back the way it came
// to close (Base UI's Drawer), by its handle, a phone's, and its title,
// or, on a touch screen, from anywhere its list is at its top; Escape,
// its close button and a tap beside it close it too, and the focus goes
// back where it was. What it holds is there while it is open,
// so it starts anew each time. initialFocus is the field the focus goes
// to when it opens, but on a touch screen when tapToType: there the sheet
// itself takes it, so the keyboard opens once the field is tapped, not
// over what the sheet lists. finalFocus is where the focus goes back to,
// when that is not what had it (a tap on a button gives it no focus in
// Safari). A tall one takes most of a phone's height, whatever it holds,
// so what it lists does not move its field up and down. A bare one leaves
// its space under the title to what it holds, which lays out what
// scrolls and what does not (its foot). One whose title follows what it
// holds (title undefined) has it there instead, in a SheetHead, which
// takes the title's place, dragged as it is. On a
// desktop, it is 440 px wide, and its head says Esc closes it.
export function Sheet({
  open,
  onOpenChange,
  title,
  initialFocus,
  tapToType = false,
  finalFocus,
  tall = false,
  bare = false,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string | undefined
  initialFocus?: RefObject<HTMLElement | null> | undefined
  tapToType?: boolean
  finalFocus?: RefObject<HTMLElement | null> | undefined
  tall?: boolean
  bare?: boolean
  children: ReactNode
}) {
  const popup = useRef<HTMLDivElement>(null)
  const desktop = useDesktop()
  // Where the SheetHead of what it holds goes, outside what it holds.
  const [head, setHead] = useState<HTMLDivElement | null>(null)

  return (
    <Drawer.Root
      open={open}
      onOpenChange={onOpenChange}
      // Back the way it came in: down on a phone, right on a desktop.
      swipeDirection={desktop ? 'right' : 'down'}
    >
      <Drawer.Portal>
        <Drawer.Backdrop className="fixed inset-0 z-40 bg-black/40 opacity-[calc(1-var(--drawer-swipe-progress,0))] transition-opacity duration-(--duration-slow) data-ending-style:opacity-0 data-starting-style:opacity-0 data-swiping:duration-0" />
        <Drawer.Viewport className="fixed inset-0 z-50">
          <Drawer.Popup
            ref={popup}
            // The toasts keep clear of an open sheet's buttons (Toasts).
            data-sheet=""
            initialFocus={
              tapToType
                ? // A tap Base UI tells as one, or a finger's screen.
                  (type) =>
                    type === 'touch' || touchScreen()
                      ? popup.current
                      : (initialFocus?.current ?? true)
                : // A dialog's way: the sheet on a tap, else its first
                  // control.
                  (initialFocus ?? ((type) => (type === 'touch' ? popup.current : true)))
            }
            finalFocus={finalFocus}
            className={cn(
              // Dragged, it follows the finger (Base UI's movement, a
              // transform, beside the translate it slides in and out by);
              // let go, it slides back or away.
              'fixed inset-x-0 bottom-0 flex max-h-[90dvh] [transform:translate(var(--drawer-swipe-movement-x,0),var(--drawer-swipe-movement-y,0))] flex-col rounded-t-xl bg-card shadow-sheet transition-transform duration-(--duration-slow) ease-sheet outline-none data-ending-style:translate-y-full data-starting-style:translate-y-full data-swiping:select-none lg:inset-y-0 lg:right-0 lg:left-auto lg:max-h-none lg:w-110 lg:rounded-l-xl lg:rounded-tr-none lg:data-ending-style:translate-x-full lg:data-ending-style:translate-y-0 lg:data-starting-style:translate-x-full lg:data-starting-style:translate-y-0',
              tall && 'h-[90dvh] lg:h-auto',
            )}
          >
            {title === undefined ? (
              <div ref={setHead} className="contents" />
            ) : (
              <Head title={title} />
            )}
            {/* A mouse selects what it holds, and drags the title only. */}
            <Drawer.Content className="flex min-h-0 flex-1 flex-col">
              <HeadSlot.Provider value={head}>
                {bare ? (
                  children
                ) : (
                  <div className="overflow-y-auto p-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
                    {children}
                  </div>
                )}
              </HeadSlot.Provider>
            </Drawer.Content>
          </Drawer.Popup>
        </Drawer.Viewport>
      </Drawer.Portal>
    </Drawer.Root>
  )
}

// HeadSlot is where a sheet without a title of its own puts its head.
const HeadSlot = createContext<HTMLElement | null>(null)

// SheetHead is a sheet's head: what a finger drags, the handle (a
// phone's) and the title, with a line under it (subtitle) when it has
// one, and the close button. A sheet without a title of its own has it in
// what it holds, which puts it in the title's place.
export function SheetHead({ title, subtitle }: { title: string; subtitle?: ReactNode }) {
  const slot = useContext(HeadSlot)
  // Once the sheet has its place: one title, mounted once.
  return slot === null ? null : createPortal(<Head title={title} subtitle={subtitle} />, slot)
}

function Head({ title, subtitle }: { title: string; subtitle?: ReactNode }) {
  const t = useMessages()
  return (
    <div className="shrink-0 border-b">
      <div
        aria-hidden
        className="mx-auto mt-2 h-1 w-10 rounded-full bg-muted-foreground/40 lg:hidden"
      />
      <div className="flex items-center gap-2 pt-1 pr-2 pb-2 pl-4 lg:py-2">
        <div className="min-w-0 flex-1">
          <Drawer.Title className="text-h3">{title}</Drawer.Title>
          {subtitle !== undefined && (
            <div className="text-small text-muted-foreground">{subtitle}</div>
          )}
        </div>
        <kbd
          aria-hidden
          className="hidden rounded-[5px] border border-control-border px-1.5 font-mono text-caption text-muted-foreground lg:inline"
        >
          Esc
        </kbd>
        <Drawer.Close
          aria-label={t.close}
          className={buttonVariants({ variant: 'ghost', size: 'icon-sm' })}
        >
          <X aria-hidden />
        </Drawer.Close>
      </div>
    </div>
  )
}
