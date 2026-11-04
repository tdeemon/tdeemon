import { useEffect, useState } from 'react'

// A minute: how often the time a page goes by is asked again.
const minute = 60_000

// useNow is the time now, for what the hour decides (the morning's
// weigh-in, the evening's card, the meal of the hour): asked again each
// minute, and as the page is shown again, so a page left open, an
// installed app's most of all, follows the clock, not the time it opened.
export function useNow(): number {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    const tick = () => {
      setNow(Date.now())
    }
    const shown = () => {
      if (document.visibilityState === 'visible') {
        tick()
      }
    }
    const timer = setInterval(tick, minute)
    document.addEventListener('visibilitychange', shown)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', shown)
    }
  }, [])
  return now
}
